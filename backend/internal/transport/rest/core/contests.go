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

// ----------------------------------------------------------------------------------
// FIXME: логика slugify не относится к транспортному слою, это нужно куда-то вынести

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

// ----------------------------------------------------------------------------------

func (h *CoreServer) CreateContest(ctx context.Context, params corev1.CreateContestParams) (*corev1.CreationResponseModel, error) {

	// FIXME: валидация не должна быть ручной
	err := validateCreateContestParams(params.Title, params.Login)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	org, err := h.organizationsUC.GetOrganizationByLogin(ctx, params.OrgLogin, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
	}

	// NOTE: сложновато, что-то где-то не додумано
	var login string
	if params.Login.IsSet() && params.Login.Value != "" {
		login = strings.ToLower(params.Login.Value)
		existing, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, login)
		if err == nil && existing.ID != uuid.Nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "contest with this login already exists in organization")
		}
	} else {
		login = h.generateUniqueContestLogin(ctx, params.OrgLogin, params.Title)
	}

	contestCreation := &models.CreateContestInput{
		OrganizationID: org.ID,
		OwnerID:        &user.Id,
		Title:          params.Title,
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

	return &corev1.CreationResponseModel{
		ID:    contestID,
		Login: corev1.NewOptString(login), // FIXME: required
	}, nil
}

func (h *CoreServer) ListOrganizationContests(ctx context.Context, params corev1.ListOrganizationContestsParams) (*corev1.ListContestsResponseModel, error) {
	// FIXME: бойлерплейт
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")

	user := middleware.GetUser(ctx)
	org, err := h.organizationsUC.GetOrganizationByLogin(ctx, params.OrgLogin, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
	}

	// FIXME: это не зона отвественности транспортного слоя
	isMember := false
	if user.Role == models.UserRoleAdmin {
		isMember = true
	} else if user.Id != uuid.Nil {
		_, err := h.organizationsUC.ListMembers(ctx, org.ID, user.Id)
		if err == nil {
			isMember = true
		}
	}

	// FIXME: тоже не зона отвественности
	visibility := ""
	if !isMember {
		visibility = "public"
	}

	contestsList, err := h.contestsUC.ListOrganizationContests(ctx, org.ID, search, visibility, page, pageSize)
	if err != nil {
		return nil, err
	}

	return ListContestsResponseDTO(contestsList), nil
}

func (h *CoreServer) GetContest(ctx context.Context, params corev1.GetContestParams) (*corev1.GetContestResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
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

	return GetContestResponseDTO(contest, ps, problemDetails, &owner), nil
}

func (h *CoreServer) UpdateContest(ctx context.Context, req *corev1.UpdateContestRequestModel, params corev1.UpdateContestParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := validateUpdateContestRequest(req)
	if err != nil {
		return err
	}

	existingContest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	var newLogin *string
	if req.Login.IsSet() && req.Login.Value != "" {
		cleaned := strings.ToLower(req.Login.Value)
		if cleaned != strings.ToLower(existingContest.Login) {
			existing, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, cleaned)
			if err == nil && existing.ID != uuid.Nil {
				return pkg.Wrap(pkg.ErrBadInput, nil, "contest with this login already exists in organization")
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
	if req.MonitorScope.IsSet() {
		settingsMap["monitor_scope"] = req.MonitorScope.Value
		hasSettingsUpdate = true
	}
	if req.SubmissionsListScope.IsSet() {
		settingsMap["submissions_list_scope"] = req.SubmissionsListScope.Value
		hasSettingsUpdate = true
	}
	if req.SubmissionsReviewScope.IsSet() {
		settingsMap["submissions_review_scope"] = req.SubmissionsReviewScope.Value
		hasSettingsUpdate = true
	}
	if req.SubmissionDetailsScope.IsSet() {
		settingsMap["submission_details_scope"] = req.SubmissionDetailsScope.Value
		hasSettingsUpdate = true
	}
	if req.FreezeDurationMinutes.IsSet() {
		if req.FreezeDurationMinutes.Null {
			delete(settingsMap, "freeze_duration_minutes")
		} else {
			settingsMap["freeze_duration_minutes"] = req.FreezeDurationMinutes.Value
		}
		hasSettingsUpdate = true
	}
	if req.FreezeStatus.IsSet() {
		settingsMap["freeze_status"] = string(req.FreezeStatus.Value)
		hasSettingsUpdate = true
	}
	if req.EnableDrafts.IsSet() {
		settingsMap["enable_drafts"] = req.EnableDrafts.Value
		hasSettingsUpdate = true
	}
	if req.EnableUpsolving.IsSet() {
		settingsMap["enable_upsolving"] = req.EnableUpsolving.Value
		hasSettingsUpdate = true
	}
	if req.EnableVirtualContests.IsSet() {
		settingsMap["enable_virtual_contests"] = req.EnableVirtualContests.Value
		hasSettingsUpdate = true
	}
	if req.ParticipationMode.IsSet() {
		settingsMap["participation_mode"] = string(req.ParticipationMode.Value)
		hasSettingsUpdate = true
	}
	if req.HideStatements.IsSet() {
		settingsMap["hide_statements"] = req.HideStatements.Value
		hasSettingsUpdate = true
	}

	var settings *map[string]interface{}
	if hasSettingsUpdate {
		settings = &settingsMap
	}

	var reqTitle, reqDesc, reqVisibility *string
	if req.Title.IsSet() {
		reqTitle = &req.Title.Value
	}
	if req.Description.IsSet() {
		reqDesc = &req.Description.Value
	}
	if req.Visibility.IsSet() {
		reqVisibility = &req.Visibility.Value
	}

	var startTime, endTime *time.Time
	if req.StartTime.IsSet() && !req.StartTime.Null {
		startTime = &req.StartTime.Value
	}
	if req.EndTime.IsSet() && !req.EndTime.Null {
		endTime = &req.EndTime.Value
	}

	err = h.contestsUC.UpdateContest(ctx, models.ContestUpdateInput{
		ID:          existingContest.ID,
		Login:       newLogin,
		Title:       reqTitle,
		Description: reqDesc,
		Visibility:  reqVisibility,
		Settings:    settings,
		StartTime:   startTime,
		EndTime:     endTime,
		OwnerID:     nil,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContest(ctx context.Context, params corev1.DeleteContestParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.contestsUC.DeleteContest(ctx, contest.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListAdminContests(ctx context.Context, params corev1.ListAdminContestsParams) (*corev1.ListContestsResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")

	var visibility *string
	if params.Visibility.IsSet() {
		v := string(params.Visibility.Value)
		visibility = &v
	}
	sortBy := "created_at"
	if params.SortBy.IsSet() {
		sortBy = string(params.SortBy.Value)
	}
	sortOrder := models.SortOrderAsc
	if params.SortOrder.IsSet() {
		sortOrder = string(params.SortOrder.Value)
	}

	filter := models.AdminContestsFilter{
		Page:       page,
		PageSize:   pageSize,
		Search:     search,
		Visibility: visibility,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
	}

	contestsList, err := h.contestsUC.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return ListContestsResponseDTO(contestsList), nil
}

func (h *CoreServer) ListUserContests(ctx context.Context, params corev1.ListUserContestsParams) (*corev1.ListUserContestsResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")

	var sortBy string
	if params.SortBy.IsSet() {
		sortBy = string(params.SortBy.Value)
	}
	var sortOrder string
	if params.SortOrder.IsSet() {
		sortOrder = string(params.SortOrder.Value)
	}

	user, err := h.usersUC.GetUserByUsername(ctx, params.Username)
	if err != nil {
		return nil, err
	}

	filter := models.UserContestsFilter{
		Page:      page,
		PageSize:  pageSize,
		UserId:    user.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListUserContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return ListUserContestsResponseDTO(contestsList), nil
}

func (h *CoreServer) ListWorkshopContests(ctx context.Context, params corev1.ListWorkshopContestsParams) (*corev1.ListContestsResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")

	user := middleware.GetUser(ctx)

	var sortBy string
	if params.SortBy.IsSet() {
		sortBy = string(params.SortBy.Value)
	}
	var sortOrder string
	if params.SortOrder.IsSet() {
		sortOrder = string(params.SortOrder.Value)
	}

	filter := models.WorkshopContestsFilter{
		Page:      page,
		PageSize:  pageSize,
		UserId:    user.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	if params.OrganizationID.IsSet() {
		filter.OrganizationID = &params.OrganizationID.Value
	}

	contestsList, err := h.contestsUC.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return ListContestsResponseDTO(contestsList), nil
}

func (h *CoreServer) ListPublicContests(ctx context.Context, params corev1.ListPublicContestsParams) (*corev1.ListContestsResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")

	var sortBy string
	if params.SortBy.IsSet() {
		sortBy = string(params.SortBy.Value)
	}
	var sortOrder string
	if params.SortOrder.IsSet() {
		sortOrder = string(params.SortOrder.Value)
	}

	filter := models.PublicContestsFilter{
		Page:      page,
		PageSize:  pageSize,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return ListContestsResponseDTO(contestsList), nil
}

func (h *CoreServer) CreateContestProblem(ctx context.Context, params corev1.CreateContestProblemParams) (*corev1.CreationResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	pkgID := uuid.Nil
	if params.PackageID.IsSet() {
		pkgID = params.PackageID.Value
	}
	err = h.contestsUC.CreateContestProblem(ctx, models.ContestProblemCreation{
		ContestId: contest.ID,
		ProblemId: params.ProblemID,
		PackageId: pkgID,
	})
	if err != nil {
		return nil, err
	}

	return &corev1.CreationResponseModel{}, nil
}

func (h *CoreServer) GetContestProblem(ctx context.Context, params corev1.GetContestProblemParams) (*corev1.GetContestProblemResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	p, err := h.contestsUC.GetContestProblem(ctx, models.ContestProblemGet{
		ContestId: contest.ID,
		ProblemId: params.ProblemID,
	})
	if err != nil {
		return nil, err
	}

	problem, err := h.problemsUC.GetProblemById(ctx, params.ProblemID)
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
			statement, samples = h.loadPackageStatementAndSamples(ctx, params.ProblemID, p.PackageID)
		}

		if statement == nil {
			statement = h.loadProblemStatement(ctx, params.ProblemID)
		}
		if len(samples) == 0 {
			samples = h.loadProblemSamples(ctx, params.ProblemID)
		}
	}

	return GetContestProblemResponseDTO(p, problem, statement, samples), nil
}

func (h *CoreServer) DownloadContestStatementsPdf(ctx context.Context, params corev1.DownloadContestStatementsPdfParams) (corev1.DownloadContestStatementsPdfOK, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return corev1.DownloadContestStatementsPdfOK{}, err
	}

	var isManager bool
	user := middleware.GetUser(ctx)
	if !user.IsGuest() {
		isManager, _ = h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	}

	if !isManager {
		if contest.GetHideStatements() {
			return corev1.DownloadContestStatementsPdfOK{}, pkg.Wrap(pkg.NoPermission, nil, "statements are hidden for this contest")
		}
		if contest.StartTime != nil && time.Now().Before(*contest.StartTime) {
			return corev1.DownloadContestStatementsPdfOK{}, pkg.Wrap(pkg.NoPermission, nil, "contest has not started yet")
		}
	}

	problems, err := h.contestsUC.GetContestProblems(ctx, contest.ID)
	if err != nil {
		return corev1.DownloadContestStatementsPdfOK{}, err
	}

	lang := params.Lang.Or("ru")

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
		return corev1.DownloadContestStatementsPdfOK{}, pkg.Wrap(pkg.ErrInternal, err, "failed to generate LaTeX booklet")
	}

	var pdfBytes []byte
	if h.bookletCompiler != nil {
		pdfBytes, err = h.bookletCompiler.CompilePDF(ctx, texSource)
	} else {
		pdfBytes, err = booklet.CompilePDFLocal(ctx, texSource)
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to compile PDF booklet", "error", err)
		return corev1.DownloadContestStatementsPdfOK{}, pkg.Wrap(pkg.ErrInternal, err, "failed to compile PDF booklet")
	}

	return corev1.DownloadContestStatementsPdfOK{
		Data: bytes.NewReader(pdfBytes),
	}, nil
}

func (h *CoreServer) DeleteContestProblem(ctx context.Context, params corev1.DeleteContestProblemParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.contestsUC.DeleteContestProblem(ctx, models.ContestProblemDeletion{
		ContestId: contest.ID,
		ProblemId: params.ProblemID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ReorderContestProblems(ctx context.Context, req *corev1.ReorderContestProblemsRequestModel, params corev1.ReorderContestProblemsParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	reorderItems := make([]models.ContestProblemReorderItem, len(req.Problems))
	for i, item := range req.Problems {
		reorderItems[i] = models.ContestProblemReorderItem{
			ProblemID: item.ProblemID,
			Position:  item.Position,
		}
	}

	err = h.contestsUC.ReorderContestProblems(ctx, contest.ID, reorderItems)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) CreateContestMember(ctx context.Context, params corev1.CreateContestMemberParams) (*corev1.CreationResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	err = h.contestsUC.CreateParticipant(ctx, models.ParticipantCreation{
		ContestId: contest.ID,
		UserId:    params.UserID,
	})
	if err != nil {
		return nil, err
	}

	return &corev1.CreationResponseModel{}, nil
}

func (h *CoreServer) DeleteContestMember(ctx context.Context, params corev1.DeleteContestMemberParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.contestsUC.DeleteParticipant(ctx, models.ParticipantDeletion{
		ContestId: contest.ID,
		UserId:    params.UserID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateContestMember(ctx context.Context, params corev1.UpdateContestMemberParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)

	// Prevent user from updating their own role
	if params.UserID == user.Id {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot update own role")
	}

	// Validate role value
	if params.Role != models.ContestRoleOwner && params.Role != models.ContestRoleModerator && params.Role != models.ContestRoleParticipant {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	err = h.contestsUC.UpdateContestMember(ctx, contest.ID, params.UserID, params.Role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListContestMembers(ctx context.Context, params corev1.ListContestMembersParams) (*corev1.ListContestMembersResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)

	participantsList, err := h.contestsUC.ListParticipants(ctx, models.ParticipantsFilter{
		Page:      page,
		PageSize:  pageSize,
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
			ContestID:   contest.ID,
			ContestRole: user.ContestRole,
			UserID:      user.UserID,
			Username:    user.Username,
			Role:        string(user.Role),
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		}
	}

	return &resp, nil
}

func (h *CoreServer) GetMyContestRole(ctx context.Context, params corev1.GetMyContestRoleParams) (*corev1.GetMyContestRoleResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	contestRole, permissionsMask, err := h.permissionsUC.GetEffectiveContestRole(ctx, contest.ID, user.Id)
	if err != nil {
		return nil, err
	}

	if contestRole == nil {
		return &corev1.GetMyContestRoleResponseModel{}, nil
	}

	permissionsMaskValue := safeInt64(permissionsMask)

	return &corev1.GetMyContestRoleResponseModel{
		Role:            *contestRole,
		PermissionsMask: corev1.NewOptInt64(permissionsMaskValue),
	}, nil
}

func (h *CoreServer) ListContestSubmissions(ctx context.Context, params corev1.ListContestSubmissionsParams) (*corev1.ListSubmissionsResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	var filterUserId *uuid.UUID
	if params.UserId.IsSet() {
		filterUserId = &params.UserId.Value
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)

	var order *int32
	if params.SortOrder.IsSet() {
		var orderVal int32
		if params.SortOrder.Value == corev1.ListContestSubmissionsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	var state *models.State
	if params.State.IsSet() {
		stateVal := models.State(params.State.Value)
		state = &stateVal
	}

	var langName *models.LanguageName
	if params.Language.IsSet() {
		lang := models.LanguageName(params.Language.Value)
		langName = &lang
	}

	var problemID *uuid.UUID
	if params.ProblemId.IsSet() {
		problemID = &params.ProblemId.Value
	}

	submissionsList, err := h.submissionsUC.ListSubmissions(ctx, models.SubmissionsFilter{
		ContestId: &contest.ID,
		Page:      page,
		PageSize:  pageSize,
		UserId:    filterUserId,
		ProblemId: problemID,
		Language:  langName,
		State:     state,
		Order:     order,
	})
	if err != nil {
		return nil, err
	}

	var since int64
	if h.natsJS != nil {
		lastSeq, seqErr := pkg.GetSubmissionsLastSequence(ctx, h.natsJS)
		if seqErr != nil {
			slog.Warn("failed to get submissions last sequence", "error", seqErr)
		} else {
			since = safeInt64(lastSeq)
		}
	}

	resp := SubmissionsListToDTO(submissionsList)
	if since != 0 {
		resp.Since = corev1.NewOptInt64(since)
	}

	return resp, nil
}

func (h *CoreServer) GetContestScoreboard(ctx context.Context, params corev1.GetContestScoreboardParams) (*corev1.ScoreboardResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
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
	if params.Unfrozen.IsSet() && params.Unfrozen.Value {
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

	return GetScoreboardResponseDTO(sb), nil
}

func (h *CoreServer) ListContestTeams(ctx context.Context, params corev1.ListContestTeamsParams) (*corev1.ListContestTeamsResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	teams, err := h.contestsUC.GetContestTeams(ctx, contest.ID, user.Id)
	if err != nil {
		return nil, err
	}

	return listContestTeamsDTO(teams), nil
}

func (h *CoreServer) CreateContestTeam(ctx context.Context, params corev1.CreateContestTeamParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)

	role := models.ContestRoleParticipant
	if params.Role.IsSet() && params.Role.Value != "" {
		role = models.ContestRole(params.Role.Value)
	}

	err = h.contestsUC.CreateContestTeam(ctx, contest.ID, params.TeamID, user.Id, role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateContestTeam(ctx context.Context, params corev1.UpdateContestTeamParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)

	err = h.contestsUC.UpdateContestTeamRole(ctx, contest.ID, params.TeamID, user.Id, models.ContestRole(params.Role))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContestTeam(ctx context.Context, params corev1.DeleteContestTeamParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)

	err = h.contestsUC.DeleteContestTeam(ctx, contest.ID, params.TeamID, user.Id)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) BlockProblemForUser(ctx context.Context, req corev1.OptBlockProblemRequestModel, params corev1.BlockProblemForUserParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)

	var reason *string
	if req.IsSet() && req.Value.Reason.IsSet() {
		reason = &req.Value.Reason.Value
	}

	err = h.contestsUC.BlockProblemForUser(ctx, contest.ID, params.UserID, params.ProblemID, reason, user.Id)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UnblockProblemForUser(ctx context.Context, params corev1.UnblockProblemForUserParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	rejudgeSubmissions := params.RejudgeSubmissions.Or(false)

	err = h.contestsUC.UnblockProblemForUser(ctx, contest.ID, params.UserID, params.ProblemID, rejudgeSubmissions)
	if err != nil {
		return err
	}

	if rejudgeSubmissions {
		filter := models.RejudgeFilter{
			ContestID: contest.ID,
			ProblemID: &params.ProblemID,
			UserID:    &params.UserID,
		}
		_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *CoreServer) GetProblemBlockStatusForUser(ctx context.Context, params corev1.GetProblemBlockStatusForUserParams) (*corev1.ProblemBlockStatusResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	block, err := h.contestsUC.GetProblemBlockStatusForUser(ctx, contest.ID, params.UserID, params.ProblemID)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return &corev1.ProblemBlockStatusResponseModel{
			IsBlocked: false,
		}, nil
	}

	return &corev1.ProblemBlockStatusResponseModel{
		IsBlocked: true,
		Reason:    stringPtrToOptNilString(block.Reason),
		CreatedAt: timePtrToOptNilDateTime(&block.CreatedAt),
	}, nil
}

// Contest Join Requests

// ListContestJoinRequests handles GET /organizations/{org_login}/contests/{contest_login}/requests
func (h *CoreServer) ListContestJoinRequests(ctx context.Context, params corev1.ListContestJoinRequestsParams) (*corev1.ListContestJoinRequestsResponseModel, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	reqs, err := h.contestsUC.ListJoinRequests(ctx, params.OrgLogin, params.ContestLogin, user.Id)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to list contest join requests")
	}

	return ListContestJoinRequestsResponseDTO(reqs), nil
}

// CreateContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests
func (h *CoreServer) CreateContestJoinRequest(ctx context.Context, req corev1.OptCreateContestJoinRequestModel, params corev1.CreateContestJoinRequestParams) (*corev1.ContestJoinRequestResponseModel, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	var message *string
	if req.IsSet() && req.Value.Message.IsSet() {
		message = &req.Value.Message.Value
	}

	joinReq, registered, err := h.contestsUC.CreateJoinRequest(ctx, params.OrgLogin, params.ContestLogin, user.Id, message)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to create contest join request")
	}

	var reqOpt corev1.OptContestJoinRequestModel
	if joinReq != nil {
		d := ContestJoinRequestDTO(*joinReq)
		reqOpt = corev1.NewOptContestJoinRequestModel(d)
	}

	return &corev1.ContestJoinRequestResponseModel{
		Registered: registered,
		Request:    reqOpt,
	}, nil
}

// GetMyContestJoinRequest handles GET /organizations/{org_login}/contests/{contest_login}/requests/mine
func (h *CoreServer) GetMyContestJoinRequest(ctx context.Context, params corev1.GetMyContestJoinRequestParams) (*corev1.ContestJoinRequestNullableResponseModel, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return &corev1.ContestJoinRequestNullableResponseModel{}, nil
	}

	joinReq, err := h.contestsUC.GetMyPendingJoinRequest(ctx, params.OrgLogin, params.ContestLogin, user.Id)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to get contest join request")
	}

	var reqOpt corev1.OptContestJoinRequestModel
	if joinReq != nil {
		d := ContestJoinRequestDTO(*joinReq)
		reqOpt = corev1.NewOptContestJoinRequestModel(d)
	}

	return &corev1.ContestJoinRequestNullableResponseModel{
		Request: reqOpt,
	}, nil
}

// CancelContestJoinRequest handles DELETE /organizations/{org_login}/contests/{contest_login}/requests/mine
func (h *CoreServer) CancelContestJoinRequest(ctx context.Context, params corev1.CancelContestJoinRequestParams) error {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.contestsUC.CancelJoinRequest(ctx, params.OrgLogin, params.ContestLogin, user.Id)
	if err != nil {
		return wrapContestUCError(err, "failed to cancel contest join request")
	}

	return nil
}

// ApproveContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests/{id}/approve
func (h *CoreServer) ApproveContestJoinRequest(ctx context.Context, params corev1.ApproveContestJoinRequestParams) error {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.contestsUC.ApproveJoinRequest(ctx, params.OrgLogin, params.ContestLogin, params.ID, user.Id)
	if err != nil {
		return wrapContestUCError(err, "failed to approve contest join request")
	}

	return nil
}

// RejectContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests/{id}/reject
func (h *CoreServer) RejectContestJoinRequest(ctx context.Context, params corev1.RejectContestJoinRequestParams) error {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.contestsUC.RejectJoinRequest(ctx, params.OrgLogin, params.ContestLogin, params.ID, user.Id)
	if err != nil {
		return wrapContestUCError(err, "failed to reject contest join request")
	}

	return nil
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
