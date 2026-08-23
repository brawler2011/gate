package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/booklet"
	"github.com/google/uuid"
)

func ordinalToLetter(ordinal int) string {
	if ordinal <= 0 {
		return "A"
	}
	res := ""
	for ordinal > 0 {
		rem := (ordinal - 1) % 26
		res = string(rune('A'+rem)) + res
		ordinal = (ordinal - 1) / 26
	}
	return res
}

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

var cyrillicToLatinMap = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "j", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	'А': "a", 'Б': "b", 'В': "v", 'Г': "g", 'Д': "d", 'Е': "e", 'Ё': "yo",
	'Ж': "zh", 'З': "z", 'И': "i", 'Й': "j", 'К': "k", 'Л': "l", 'М': "m",
	'Н': "n", 'О': "o", 'П': "p", 'Р': "r", 'С': "s", 'Т': "t", 'У': "u",
	'Ф': "f", 'Х': "kh", 'Ц': "ts", 'Ч': "ch", 'Ш': "sh", 'Щ': "shch",
	'Ъ': "", 'Ы': "y", 'Ь': "", 'Э': "e", 'Ю': "yu", 'Я': "ya",
	'і': "i", 'І': "i", 'ї': "yi", 'Ї': "yi", 'є': "ye", 'Є': "ye", 'ґ': "g", 'Ґ': "g",
	'ў': "u", 'Ў': "u",
}

func transliterateCyrillic(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if tr, ok := cyrillicToLatinMap[r]; ok {
			sb.WriteString(tr)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func slugifyContestTitle(title string) string {
	transliterated := transliterateCyrillic(title)
	slug := strings.ToLower(transliterated)
	slug = nonAlphaNumRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) < 3 {
		slug = "contest-" + uuid.New().String()[:8]
	}
	if len(slug) > 64 {
		slug = strings.TrimRight(slug[:64], "-")
		if len(slug) < 3 {
			slug = "contest-" + uuid.New().String()[:8]
		}
	}
	return slug
}

func (h *CoreServer) generateUniqueContestLogin(ctx context.Context, orgLogin, title string) string {
	baseSlug := slugifyContestTitle(title)
	slug := baseSlug
	suffix := 1

	for {
		if !isReservedContestLogin(slug) {
			existing, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, slug)
			if err != nil || existing.ID == uuid.Nil {
				return slug
			}
		}
		suffix++
		suffixStr := fmt.Sprintf("-%d", suffix)
		maxBaseLen := 64 - len(suffixStr)
		if len(baseSlug) > maxBaseLen {
			slug = strings.TrimRight(baseSlug[:maxBaseLen], "-") + suffixStr
		} else {
			slug = baseSlug + suffixStr
		}
	}
}

func (h *CoreServer) CreateContest(ctx context.Context, request corev1.CreateContestRequestObject) (corev1.CreateContestResponseObject, error) {
	err := validateCreateContestParams(request.Params)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	org, err := h.organizationsUC.GetOrganizationByLogin(ctx, request.OrgLogin, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
	}

	var login string
	if request.Params.Login != nil && *request.Params.Login != "" {
		login = strings.ToLower(*request.Params.Login)
		existing, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, login)
		if err == nil && existing.ID != uuid.Nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "contest with this login already exists in organization")
		}
	} else {
		login = h.generateUniqueContestLogin(ctx, request.OrgLogin, request.Params.Title)
	}

	contestCreation := &models.CreateContestInput{
		OrganizationID: org.ID,
		OwnerID:        &user.Id,
		Title:          request.Params.Title,
		Login:          login,
		Description:    "",
		Visibility:     models.ContestVisibilityPrivate,
		Settings:       make(map[string]interface{}),
		StartTime:      nil,
		EndTime:        nil,
	}

	contestID, err := h.contestsUC.CreateContest(ctx, contestCreation)
	if err != nil {
		return nil, err
	}

	return corev1.CreateContest200JSONResponse{Id: contestID, Login: &login}, nil
}

func (h *CoreServer) ListOrganizationContests(ctx context.Context, request corev1.ListOrganizationContestsRequestObject) (corev1.ListOrganizationContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	org, err := h.organizationsUC.GetOrganizationByLogin(ctx, request.OrgLogin, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	isMember := false
	if user.Role == models.UserRoleAdmin {
		isMember = true
	} else if user.Id != uuid.Nil {
		_, err := h.organizationsUC.ListMembers(ctx, org.ID, user.Id)
		if err == nil {
			isMember = true
		}
	}

	visibility := ""
	if !isMember {
		visibility = "public"
	}

	contestsList, err := h.contestsUC.ListOrganizationContests(ctx, org.ID, search, visibility, request.Params.Page, request.Params.PageSize)
	if err != nil {
		return nil, err
	}

	return corev1.ListOrganizationContests200JSONResponse(*ListContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) GetContest(ctx context.Context, request corev1.GetContestRequestObject) (corev1.GetContestResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	allowed, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionGetContestProblem)
	if err != nil {
		return nil, err
	}

	var ps []models.ContestProblem
	if allowed {
		ps, err = h.contestsUC.GetContestProblems(ctx, contest.ID)
		if err != nil {
			return nil, err
		}
	}

	problemDetails := make(map[uuid.UUID]models.Problem, len(ps))
	for _, p := range ps {
		prob, err := h.problemsUC.GetProblemById(ctx, p.ProblemID)
		if err != nil {
			continue
		}
		problemDetails[p.ProblemID] = prob
	}

	var owner models.User
	if contest.OwnerID != nil {
		owner, err = h.usersUC.GetUserById(ctx, *contest.OwnerID)
		if err != nil {
			return nil, err
		}
	}

	return corev1.GetContest200JSONResponse(*GetContestResponseDTO(contest, ps, problemDetails, &owner)), nil
}

func (h *CoreServer) UpdateContest(ctx context.Context, request corev1.UpdateContestRequestObject) (corev1.UpdateContestResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}
	req := *request.Body

	err := validateUpdateContestRequest(req)
	if err != nil {
		return nil, err
	}

	existingContest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	var newLogin *string
	if req.Login != nil && *req.Login != "" {
		cleaned := strings.ToLower(*req.Login)
		if cleaned != strings.ToLower(existingContest.Login) {
			existing, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, cleaned)
			if err == nil && existing.ID != uuid.Nil {
				return nil, pkg.Wrap(pkg.ErrBadInput, nil, "contest with this login already exists in organization")
			}
			newLogin = &cleaned
		}
	}

	settingsMap := make(map[string]interface{})
	if existingContest.Settings != nil {
		for k, v := range existingContest.Settings {
			settingsMap[k] = v
		}
	}

	hasSettingsUpdate := false
	if req.MonitorScope != nil {
		settingsMap["monitor_scope"] = *req.MonitorScope
		hasSettingsUpdate = true
	}
	if req.SubmissionsListScope != nil {
		settingsMap["submissions_list_scope"] = *req.SubmissionsListScope
		hasSettingsUpdate = true
	}
	if req.SubmissionsReviewScope != nil {
		settingsMap["submissions_review_scope"] = *req.SubmissionsReviewScope
		hasSettingsUpdate = true
	}
	if req.SubmissionDetailsScope != nil {
		settingsMap["submission_details_scope"] = *req.SubmissionDetailsScope
		hasSettingsUpdate = true
	}
	if req.FreezeDurationMinutes != nil {
		settingsMap["freeze_duration_minutes"] = *req.FreezeDurationMinutes
		hasSettingsUpdate = true
	}
	if req.FreezeStatus != nil {
		settingsMap["freeze_status"] = string(*req.FreezeStatus)
		hasSettingsUpdate = true
	}
	if req.EnableDrafts != nil {
		settingsMap["enable_drafts"] = *req.EnableDrafts
		hasSettingsUpdate = true
	}
	if req.EnableUpsolving != nil {
		settingsMap["enable_upsolving"] = *req.EnableUpsolving
		hasSettingsUpdate = true
	}
	if req.EnableVirtualContests != nil {
		settingsMap["enable_virtual_contests"] = *req.EnableVirtualContests
		hasSettingsUpdate = true
	}
	if req.ParticipationMode != nil {
		settingsMap["participation_mode"] = string(*req.ParticipationMode)
		hasSettingsUpdate = true
	}
	if req.HideStatements != nil {
		settingsMap["hide_statements"] = *req.HideStatements
		hasSettingsUpdate = true
	}

	var settings *map[string]interface{}
	if hasSettingsUpdate {
		settings = &settingsMap
	}

	err = h.contestsUC.UpdateContest(ctx, models.ContestUpdateInput{
		ID:          existingContest.ID,
		Login:       newLogin,
		Title:       req.Title,
		Description: req.Description,
		Visibility:  req.Visibility,
		Settings:    settings,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		OwnerID:     nil,
	})
	if err != nil {
		return nil, err
	}

	return corev1.UpdateContest200Response{}, nil
}

func (h *CoreServer) DeleteContest(ctx context.Context, request corev1.DeleteContestRequestObject) (corev1.DeleteContestResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	err = h.contestsUC.DeleteContest(ctx, contest.ID)
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContest200Response{}, nil
}

func (h *CoreServer) ListAdminContests(ctx context.Context, request corev1.ListAdminContestsRequestObject) (corev1.ListAdminContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	var visibility *string
	if request.Params.Visibility != nil {
		v := string(*request.Params.Visibility)
		visibility = &v
	}
	sortBy := "created_at"
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	sortOrder := models.SortOrderAsc
	if request.Params.SortOrder != nil {
		s := string(*request.Params.SortOrder)
		sortOrder = s
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	filter := models.AdminContestsFilter{
		Page:       request.Params.Page,
		PageSize:   request.Params.PageSize,
		Search:     search,
		Visibility: visibility,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
	}

	contestsList, err := h.contestsUC.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListAdminContests200JSONResponse(*ListContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) ListUserContests(ctx context.Context, request corev1.ListUserContestsRequestObject) (corev1.ListUserContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}
	var sortBy string
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	var sortOrder string
	if request.Params.SortOrder != nil {
		sortOrder = string(*request.Params.SortOrder)
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	user, err := h.usersUC.GetUserByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}

	filter := models.UserContestsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		UserId:    user.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListUserContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListUserContests200JSONResponse(*ListUserContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) ListWorkshopContests(ctx context.Context, request corev1.ListWorkshopContestsRequestObject) (corev1.ListWorkshopContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	var sortBy string
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	var sortOrder string
	if request.Params.SortOrder != nil {
		sortOrder = string(*request.Params.SortOrder)
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	filter := models.WorkshopContestsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		UserId:    user.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	if request.Params.OrganizationId != nil {
		orgID, err := uuid.Parse(request.Params.OrganizationId.String())
		if err == nil {
			filter.OrganizationID = &orgID
		}
	}

	contestsList, err := h.contestsUC.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListWorkshopContests200JSONResponse(*ListContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) ListPublicContests(ctx context.Context, request corev1.ListPublicContestsRequestObject) (corev1.ListPublicContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	var sortBy string
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	var sortOrder string
	if request.Params.SortOrder != nil {
		sortOrder = string(*request.Params.SortOrder)
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	filter := models.PublicContestsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListPublicContests200JSONResponse(*ListContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) CreateContestProblem(ctx context.Context, request corev1.CreateContestProblemRequestObject) (corev1.CreateContestProblemResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	pkgID := uuid.Nil
	if request.Params.PackageId != nil {
		pkgID = *request.Params.PackageId
	}
	err = h.contestsUC.CreateContestProblem(ctx, models.ContestProblemCreation{
		ContestId: contest.ID,
		ProblemId: request.Params.ProblemId,
		PackageId: pkgID,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestProblem200JSONResponse{}, nil
}

func (h *CoreServer) GetContestProblem(ctx context.Context, request corev1.GetContestProblemRequestObject) (corev1.GetContestProblemResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	p, err := h.contestsUC.GetContestProblem(ctx, models.ContestProblemGet{
		ContestId: contest.ID,
		ProblemId: request.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	problem, err := h.problemsUC.GetProblemById(ctx, request.ProblemId)
	if err != nil {
		return nil, err
	}

	var isManager bool
	user := middleware.GetUser(ctx)
	if !user.IsGuest() {
		isManager, _ = h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	}

	var statement *models.Statement
	var samples []corev1.ProblemSampleModel

	if !contest.GetHideStatements() || isManager {
		if p.PackageID != uuid.Nil {
			statement, samples = h.loadPackageStatementAndSamples(ctx, request.ProblemId, p.PackageID)
		}

		if statement == nil {
			statement = h.loadProblemStatement(ctx, request.ProblemId)
		}
		if len(samples) == 0 {
			samples = h.loadProblemSamples(ctx, request.ProblemId)
		}
	}

	return corev1.GetContestProblem200JSONResponse(*GetContestProblemResponseDTO(p, problem, statement, samples)), nil
}

func (h *CoreServer) DownloadContestStatementsPdf(ctx context.Context, request corev1.DownloadContestStatementsPdfRequestObject) (corev1.DownloadContestStatementsPdfResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	var isManager bool
	user := middleware.GetUser(ctx)
	if !user.IsGuest() {
		isManager, _ = h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	}

	if !isManager {
		if contest.GetHideStatements() {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "statements are hidden for this contest")
		}
		if contest.StartTime != nil && time.Now().Before(*contest.StartTime) {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "contest has not started yet")
		}
	}

	problems, err := h.contestsUC.GetContestProblems(ctx, contest.ID)
	if err != nil {
		return nil, err
	}

	lang := "ru"
	if request.Params.Lang != nil && *request.Params.Lang != "" {
		lang = *request.Params.Lang
	}

	var problemDatas []booklet.ProblemData
	for _, cp := range problems {
		prob, err := h.problemsUC.GetProblemById(ctx, cp.ProblemID)
		if err != nil {
			continue
		}

		var statement *models.Statement
		var samples []corev1.ProblemSampleModel

		if cp.PackageID != uuid.Nil {
			statement, samples = h.loadPackageStatementAndSamplesWithLang(ctx, cp.ProblemID, cp.PackageID, lang)
		}
		if statement == nil {
			statement = h.loadProblemStatementWithLang(ctx, cp.ProblemID, lang)
		}
		if len(samples) == 0 {
			samples = h.loadProblemSamples(ctx, cp.ProblemID)
		}

		title := strings.TrimSpace(prob.Title)
		if title == "" {
			title = strings.TrimSpace(cp.Title)
		}
		if statement != nil && strings.TrimSpace(statement.Title) != "" {
			title = strings.TrimSpace(statement.Title)
		}

		var bookletSamples []booklet.SampleData
		for _, s := range samples {
			bookletSamples = append(bookletSamples, booklet.SampleData{
				Input:  s.Input,
				Output: s.Output,
			})
		}

		legend := ""
		inputFormat := ""
		outputFormat := ""
		interaction := ""
		scoring := ""
		notes := ""
		if statement != nil {
			legend = statement.Legend
			inputFormat = statement.InputFormat
			outputFormat = statement.OutputFormat
			interaction = statement.Interaction
			scoring = statement.Scoring
			notes = statement.Notes
		}

		problemDatas = append(problemDatas, booklet.ProblemData{
			Letter:        ordinalToLetter(cp.Ordinal),
			Title:         title,
			TimeLimitMs:   prob.TimeLimitMs,
			MemoryLimitMb: prob.MemoryLimitMb,
			InputFile:     "stdin",
			OutputFile:    "stdout",
			Legend:        legend,
			InputFormat:   inputFormat,
			OutputFormat:  outputFormat,
			Interaction:   interaction,
			Scoring:       scoring,
			Notes:         notes,
			Samples:       bookletSamples,
		})
	}

	dateStr := time.Now().Format("02.01.2006")
	if contest.StartTime != nil {
		dateStr = contest.StartTime.Format("02.01.2006")
	}

	contestData := booklet.ContestData{
		Title:        contest.Title,
		Organization: contest.OrganizationLogin,
		Date:         dateStr,
		Language:     lang,
		Problems:     problemDatas,
	}

	texSource, err := booklet.GenerateLatex(contestData)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to generate LaTeX booklet")
	}

	var pdfBytes []byte
	if h.bookletCompiler != nil {
		pdfBytes, err = h.bookletCompiler.CompilePDF(ctx, texSource)
	} else {
		pdfBytes, err = booklet.CompilePDFLocal(ctx, texSource)
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to compile PDF booklet", "error", err)
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to compile PDF booklet")
	}

	return corev1.DownloadContestStatementsPdf200ApplicationpdfResponse{
		Body:          bytes.NewReader(pdfBytes),
		ContentLength: int64(len(pdfBytes)),
	}, nil
}

func (h *CoreServer) DeleteContestProblem(ctx context.Context, request corev1.DeleteContestProblemRequestObject) (corev1.DeleteContestProblemResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	err = h.contestsUC.DeleteContestProblem(ctx, models.ContestProblemDeletion{
		ContestId: contest.ID,
		ProblemId: request.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContestProblem200Response{}, nil
}

func (h *CoreServer) ReorderContestProblems(ctx context.Context, request corev1.ReorderContestProblemsRequestObject) (corev1.ReorderContestProblemsResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	reorderItems := make([]models.ContestProblemReorderItem, len(request.Body.Problems))
	for i, item := range request.Body.Problems {
		reorderItems[i] = models.ContestProblemReorderItem{
			ProblemID: item.ProblemId,
			Position:  item.Position,
		}
	}

	err = h.contestsUC.ReorderContestProblems(ctx, contest.ID, reorderItems)
	if err != nil {
		return nil, err
	}

	return corev1.ReorderContestProblems200Response{}, nil
}

func (h *CoreServer) CreateContestMember(ctx context.Context, request corev1.CreateContestMemberRequestObject) (corev1.CreateContestMemberResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	err = h.contestsUC.CreateParticipant(ctx, models.ParticipantCreation{
		ContestId: contest.ID,
		UserId:    request.Params.UserId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestMember200JSONResponse{}, nil
}

func (h *CoreServer) DeleteContestMember(ctx context.Context, request corev1.DeleteContestMemberRequestObject) (corev1.DeleteContestMemberResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	err = h.contestsUC.DeleteParticipant(ctx, models.ParticipantDeletion{
		ContestId: contest.ID,
		UserId:    request.Params.UserId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContestMember200Response{}, nil
}

func (h *CoreServer) UpdateContestMember(ctx context.Context, request corev1.UpdateContestMemberRequestObject) (corev1.UpdateContestMemberResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	userId, err := uuid.Parse(request.Params.UserId.String())
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid user_id")
	}

	// Prevent user from updating their own role
	if userId == user.Id {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "cannot update own role")
	}

	// Validate role value
	if request.Params.Role != models.ContestRoleOwner && request.Params.Role != models.ContestRoleModerator && request.Params.Role != models.ContestRoleParticipant {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	err = h.contestsUC.UpdateContestMember(ctx, contest.ID, userId, request.Params.Role)
	if err != nil {
		return nil, err
	}

	return corev1.UpdateContestMember200Response{}, nil
}

func (h *CoreServer) ListContestMembers(ctx context.Context, request corev1.ListContestMembersRequestObject) (corev1.ListContestMembersResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	participantsList, err := h.contestsUC.ListParticipants(ctx, models.ParticipantsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		ContestId: contest.ID,
	})
	if err != nil {
		return nil, err
	}

	resp := corev1.ListContestMembersResponseModel{
		Members:    make([]corev1.ContestMemberModel, len(participantsList.Members)),
		Pagination: PaginationDTO(participantsList.Pagination),
	}

	for i, user := range participantsList.Members {
		resp.Members[i] = corev1.ContestMemberModel{
			ContestId:   contest.ID,
			ContestRole: user.ContestRole,
			UserId:      user.UserID,
			Username:    user.Username,
			Role:        string(user.Role),
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		}
	}

	return corev1.ListContestMembers200JSONResponse(resp), nil
}

func (h *CoreServer) GetMyContestRole(ctx context.Context, request corev1.GetMyContestRoleRequestObject) (corev1.GetMyContestRoleResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	contestRole, permissionsMask, err := h.permissionsUC.GetEffectiveContestRole(ctx, contest.ID, user.Id)
	if err != nil {
		return nil, err
	}

	if contestRole == nil {
		return corev1.GetMyContestRole200JSONResponse{}, nil
	}

	permissionsMaskValue := safeInt64(permissionsMask)

	return corev1.GetMyContestRole200JSONResponse{
		Role:            *contestRole,
		PermissionsMask: &permissionsMaskValue,
	}, nil
}

func (h *CoreServer) ListContestSubmissions(ctx context.Context, request corev1.ListContestSubmissionsRequestObject) (corev1.ListContestSubmissionsResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	filterUserId := request.Params.UserId

	var order *int32
	if request.Params.SortOrder != nil {
		var orderVal int32
		if *request.Params.SortOrder == corev1.ListContestSubmissionsParamsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	var state *models.State
	if request.Params.State != nil {
		stateVal := models.State(*request.Params.State)
		state = &stateVal
	}

	var langName *models.LanguageName
	if request.Params.Language != nil {
		lang := models.LanguageName(*request.Params.Language)
		langName = &lang
	}

	submissionsList, err := h.submissionsUC.ListSubmissions(ctx, models.SubmissionsFilter{
		ContestId: &contest.ID,
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		UserId:    filterUserId,
		ProblemId: request.Params.ProblemId,
		Language:  langName,
		State:     state,
		Order:     order,
	})
	if err != nil {
		return nil, err
	}

	var since int64
	lastSeq, seqErr := pkg.GetSubmissionsLastSequence(ctx, h.natsJS)
	if seqErr != nil {
		slog.Warn("failed to get submissions last sequence", "error", seqErr)
	} else {
		since = safeInt64(lastSeq)
	}

	resp := *ListSolutionsResponseDTO(submissionsList)
	resp.Since = &since

	return corev1.ListContestSubmissions200JSONResponse(resp), nil
}

func (h *CoreServer) GetContestScoreboard(ctx context.Context, request corev1.GetContestScoreboardRequestObject) (corev1.GetContestScoreboardResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	allowed, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionGetMonitor)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "permission denied to view contest monitor")
	}

	isManager := user.Role == models.UserRoleAdmin || (contest.OwnerID != nil && *contest.OwnerID == user.Id)
	hasStarted := contest.StartTime == nil || !time.Now().Before(*contest.StartTime)
	if !isManager && !hasStarted {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "contest has not started yet")
	}

	unfrozen := false
	if request.Params.Unfrozen != nil && *request.Params.Unfrozen {
		unfrozen = true
		canManage, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
		if err != nil {
			return nil, err
		}
		if !canManage {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "permission denied to view unfrozen scoreboard")
		}
	}

	sb, err := h.contestsUC.GetContestScoreboard(ctx, contest.ID, user.Id, unfrozen)
	if err != nil {
		return nil, err
	}

	return corev1.GetContestScoreboard200JSONResponse(*GetScoreboardResponseDTO(sb)), nil
}

func (h *CoreServer) ListContestTeams(ctx context.Context, request corev1.ListContestTeamsRequestObject) (corev1.ListContestTeamsResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	teams, err := h.contestsUC.GetContestTeams(ctx, contest.ID, user.Id)
	if err != nil {
		return nil, err
	}

	return corev1.ListContestTeams200JSONResponse(*listContestTeamsDTO(teams)), nil
}

func (h *CoreServer) CreateContestTeam(ctx context.Context, request corev1.CreateContestTeamRequestObject) (corev1.CreateContestTeamResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	role := models.ContestRoleParticipant
	if request.Params.Role != nil && *request.Params.Role != "" {
		role = models.ContestRole(*request.Params.Role)
	}

	err = h.contestsUC.CreateContestTeam(ctx, contest.ID, request.Params.TeamId, user.Id, role)
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestTeam200Response{}, nil
}

func (h *CoreServer) UpdateContestTeam(ctx context.Context, request corev1.UpdateContestTeamRequestObject) (corev1.UpdateContestTeamResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	err = h.contestsUC.UpdateContestTeamRole(ctx, contest.ID, request.Params.TeamId, user.Id, models.ContestRole(request.Params.Role))
	if err != nil {
		return nil, err
	}

	return corev1.UpdateContestTeam200Response{}, nil
}

func (h *CoreServer) DeleteContestTeam(ctx context.Context, request corev1.DeleteContestTeamRequestObject) (corev1.DeleteContestTeamResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	err = h.contestsUC.DeleteContestTeam(ctx, contest.ID, request.Params.TeamId, user.Id)
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContestTeam200Response{}, nil
}

func (h *CoreServer) BlockProblemForUser(ctx context.Context, request corev1.BlockProblemForUserRequestObject) (corev1.BlockProblemForUserResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	var reason *string
	if request.Body != nil && request.Body.Reason != nil {
		reason = request.Body.Reason
	}

	err = h.contestsUC.BlockProblemForUser(ctx, contest.ID, request.UserId, request.ProblemId, reason, user.Id)
	if err != nil {
		return nil, err
	}

	return corev1.BlockProblemForUser200Response{}, nil
}

func (h *CoreServer) UnblockProblemForUser(ctx context.Context, request corev1.UnblockProblemForUserRequestObject) (corev1.UnblockProblemForUserResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	rejudgeSubmissions := request.Params.RejudgeSubmissions != nil && *request.Params.RejudgeSubmissions

	err = h.contestsUC.UnblockProblemForUser(ctx, contest.ID, request.UserId, request.ProblemId, rejudgeSubmissions)
	if err != nil {
		return nil, err
	}

	if rejudgeSubmissions {
		filter := models.RejudgeFilter{
			ContestID: contest.ID,
			ProblemID: &request.ProblemId,
			UserID:    &request.UserId,
		}
		_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
		if err != nil {
			return nil, err
		}
	}

	return corev1.UnblockProblemForUser200Response{}, nil
}

func (h *CoreServer) GetProblemBlockStatusForUser(ctx context.Context, request corev1.GetProblemBlockStatusForUserRequestObject) (corev1.GetProblemBlockStatusForUserResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	block, err := h.contestsUC.GetProblemBlockStatusForUser(ctx, contest.ID, request.UserId, request.ProblemId)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return corev1.GetProblemBlockStatusForUser200JSONResponse{
			IsBlocked: false,
		}, nil
	}

	createdAt := block.CreatedAt
	return corev1.GetProblemBlockStatusForUser200JSONResponse{
		IsBlocked: true,
		Reason:    block.Reason,
		CreatedAt: &createdAt,
	}, nil
}

// Contest Join Requests

// ListContestJoinRequests handles GET /organizations/{org_login}/contests/{contest_login}/requests
func (h *CoreServer) ListContestJoinRequests(ctx context.Context, request corev1.ListContestJoinRequestsRequestObject) (corev1.ListContestJoinRequestsResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	reqs, err := h.contestsUC.ListJoinRequests(ctx, request.OrgLogin, request.ContestLogin, user.Id)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to list contest join requests")
	}

	return corev1.ListContestJoinRequests200JSONResponse(*ListContestJoinRequestsResponseDTO(reqs)), nil
}

// CreateContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests
func (h *CoreServer) CreateContestJoinRequest(ctx context.Context, request corev1.CreateContestJoinRequestRequestObject) (corev1.CreateContestJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	var message *string
	if request.Body != nil {
		message = request.Body.Message
	}

	req, registered, err := h.contestsUC.CreateJoinRequest(ctx, request.OrgLogin, request.ContestLogin, user.Id, message)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to create contest join request")
	}

	var reqDTO *corev1.ContestJoinRequestModel
	if req != nil {
		d := ContestJoinRequestDTO(*req)
		reqDTO = &d
	}

	return corev1.CreateContestJoinRequest200JSONResponse{
		Registered: registered,
		Request:    reqDTO,
	}, nil
}

// GetMyContestJoinRequest handles GET /organizations/{org_login}/contests/{contest_login}/requests/mine
func (h *CoreServer) GetMyContestJoinRequest(ctx context.Context, request corev1.GetMyContestJoinRequestRequestObject) (corev1.GetMyContestJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return corev1.GetMyContestJoinRequest200JSONResponse{Request: nil}, nil
	}

	req, err := h.contestsUC.GetMyPendingJoinRequest(ctx, request.OrgLogin, request.ContestLogin, user.Id)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to get contest join request")
	}

	var reqDTO *corev1.ContestJoinRequestModel
	if req != nil {
		d := ContestJoinRequestDTO(*req)
		reqDTO = &d
	}

	return corev1.GetMyContestJoinRequest200JSONResponse{
		Request: reqDTO,
	}, nil
}

// CancelContestJoinRequest handles DELETE /organizations/{org_login}/contests/{contest_login}/requests/mine
func (h *CoreServer) CancelContestJoinRequest(ctx context.Context, request corev1.CancelContestJoinRequestRequestObject) (corev1.CancelContestJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.contestsUC.CancelJoinRequest(ctx, request.OrgLogin, request.ContestLogin, user.Id)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to cancel contest join request")
	}

	return corev1.CancelContestJoinRequest200Response{}, nil
}

// ApproveContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests/{id}/approve
func (h *CoreServer) ApproveContestJoinRequest(ctx context.Context, request corev1.ApproveContestJoinRequestRequestObject) (corev1.ApproveContestJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.contestsUC.ApproveJoinRequest(ctx, request.OrgLogin, request.ContestLogin, request.Id, user.Id)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to approve contest join request")
	}

	return corev1.ApproveContestJoinRequest200Response{}, nil
}

// RejectContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests/{id}/reject
func (h *CoreServer) RejectContestJoinRequest(ctx context.Context, request corev1.RejectContestJoinRequestRequestObject) (corev1.RejectContestJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.contestsUC.RejectJoinRequest(ctx, request.OrgLogin, request.ContestLogin, request.Id, user.Id)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to reject contest join request")
	}

	return corev1.RejectContestJoinRequest200Response{}, nil
}

func wrapContestUCError(err error, fallbackMsg string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pkg.NoPermission) || strings.Contains(err.Error(), "access denied") {
		return pkg.Wrap(pkg.NoPermission, err, fallbackMsg)
	}
	if errors.Is(err, pkg.ErrNotFound) {
		return pkg.Wrap(pkg.ErrNotFound, err, fallbackMsg)
	}
	if errors.Is(err, pkg.ErrBadInput) {
		return pkg.Wrap(pkg.ErrBadInput, err, fallbackMsg)
	}
	return pkg.Wrap(pkg.ErrInternal, err, fallbackMsg)
}



