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

	paging "github.com/brawler2011/gate/backend/internal/page"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

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
			existing, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, slug)
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

func (h *CoreServer) CreateContest(ctx context.Context, params corev1.CreateContestParams) (*corev1.CreationResponseModel, error) {

	// FIXME: валидация не должна быть ручной
	err := validateCreateContestParams(params.Title, params.Login)
	if err != nil {
		return nil, err
	}

	user := accessmodel.UserFromContext(ctx)

	org, err := h.Organizations.GetOrganizationByLogin(ctx, params.OrgLogin)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
	}

	// NOTE: сложновато, что-то где-то не додумано
	var login string
	if params.Login.IsSet() && params.Login.Value != "" {
		login = strings.ToLower(params.Login.Value)
		existing, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, login)
		if err == nil && existing.ID != uuid.Nil {
			return nil, pkg.Wrap(pkg.ErrConflict, nil, "contest with this login already exists in organization")
		}
	} else {
		login = h.generateUniqueContestLogin(ctx, params.OrgLogin, params.Title)
	}

	contestCreation := &contestsmodel.CreateContestInput{
		OrganizationID: org.ID,
		OwnerID:        &user.Id,
		Title:          params.Title,
		Login:          login,
		Description:    "",
		Visibility:     contestsmodel.ContestVisibilityPrivate,
		Settings:       make(map[string]interface{}),
		StartTime:      nil,
		EndTime:        nil,
	}

	contestID, err := h.Contests.CreateContest(ctx, contestCreation)
	if err != nil {
		return nil, err
	}

	return &corev1.CreationResponseModel{
		ID:    contestID,
		Login: corev1.NewOptString(login), // FIXME: required
	}, nil
}

func (h *CoreServer) ListOrganizationContests(ctx context.Context, params corev1.ListOrganizationContestsParams) (*corev1.ListContestsResponseModel, error) {
	list, err := h.Contests.ListOrganizationContests(ctx, params.OrgLogin, params.Search.Or(""), params.Page.Or(1), params.PageSize.Or(50))
	if err != nil {
		return nil, err
	}
	return projectResponse(h, ctx, ListContestsResponseDTO(list))
}

func (h *CoreServer) GetContest(ctx context.Context, params corev1.GetContestParams) (*corev1.GetContestResponseModel, error) {
	details, err := h.ContestDetails.GetContestDetails(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}
	return projectResponse(h, ctx, GetContestResponseDTO(details.Contest, details.Problems, details.Owner))
}

func (h *CoreServer) UpdateContest(ctx context.Context, req *corev1.UpdateContestRequestModel, params corev1.UpdateContestParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := validateUpdateContestRequest(req)
	if err != nil {
		return err
	}

	existingContest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	var newLogin *string
	if req.Login.IsSet() && req.Login.Value != "" {
		cleaned := strings.ToLower(req.Login.Value)
		if cleaned != strings.ToLower(existingContest.Login) {
			existing, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, cleaned)
			if err == nil && existing.ID != uuid.Nil {
				return pkg.Wrap(pkg.ErrConflict, nil, "contest with this login already exists in organization")
			}
			newLogin = &cleaned
		}
	}

	settingsMap := make(map[string]interface{})

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
			settingsMap["freeze_duration_minutes"] = nil
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
	if req.AllowClarifications.IsSet() {
		settingsMap["allow_clarifications"] = req.AllowClarifications.Value
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

	err = h.Contests.UpdateContest(ctx, contestsmodel.ContestUpdateInput{
		ID:           existingContest.ID,
		Login:        newLogin,
		Title:        reqTitle,
		Description:  reqDesc,
		Visibility:   reqVisibility,
		Settings:     settings,
		StartTime:    startTime,
		StartTimeSet: req.StartTime.IsSet(),
		EndTime:      endTime,
		EndTimeSet:   req.EndTime.IsSet(),
		OwnerID:      nil,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContest(ctx context.Context, params corev1.DeleteContestParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.Contests.DeleteContest(ctx, contest.ID)
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
	sortOrder := paging.SortOrderDesc
	if params.SortOrder.IsSet() {
		sortOrder = string(params.SortOrder.Value)
	}

	filter := contestsmodel.AdminContestsFilter{
		Page:       page,
		PageSize:   pageSize,
		Search:     search,
		Visibility: visibility,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
	}

	contestsList, err := h.Contests.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, ListContestsResponseDTO(contestsList))
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

	user, err := h.UserReads.GetUserProfileByUsername(ctx, params.Username)
	if err != nil {
		return nil, err
	}

	filter := contestsmodel.UserContestsFilter{
		Page:      page,
		PageSize:  pageSize,
		UserId:    user.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.Contests.ListUserContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, ListUserContestsResponseDTO(contestsList))
}

func (h *CoreServer) ListWorkshopContests(ctx context.Context, params corev1.ListWorkshopContestsParams) (*corev1.ListContestsResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")

	user := accessmodel.UserFromContext(ctx)

	var sortBy string
	if params.SortBy.IsSet() {
		sortBy = string(params.SortBy.Value)
	}
	var sortOrder string
	if params.SortOrder.IsSet() {
		sortOrder = string(params.SortOrder.Value)
	}

	filter := contestsmodel.WorkshopContestsFilter{
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

	contestsList, err := h.Contests.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, ListContestsResponseDTO(contestsList))
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

	filter := contestsmodel.PublicContestsFilter{
		Page:      page,
		PageSize:  pageSize,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.Contests.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, ListContestsResponseDTO(contestsList))
}

func (h *CoreServer) CreateContestProblem(ctx context.Context, params corev1.CreateContestProblemParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	pkgID := uuid.Nil
	if params.PackageID.IsSet() {
		pkgID = params.PackageID.Value
	}
	err = h.Contests.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{
		ContestId: contest.ID,
		ProblemId: params.ProblemID,
		PackageId: pkgID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) GetContestProblem(ctx context.Context, params corev1.GetContestProblemParams) (*corev1.GetContestProblemResponseModel, error) {
	details, err := h.ContestDetails.GetContestProblemDetails(ctx, params.OrgLogin, params.ContestLogin, params.ProblemID)
	if err != nil {
		return nil, err
	}
	return projectResponse(h, ctx, GetContestProblemResponseDTO(details.Link, details.Problem, details.Statement, SamplesDTO(details.Samples)), details.Link.ContestID)
}

func (h *CoreServer) DownloadContestStatementsPdf(ctx context.Context, params corev1.DownloadContestStatementsPdfParams) (corev1.DownloadContestStatementsPdfOK, error) {
	data, err := h.ContestDetails.ReadBooklet(ctx, params.OrgLogin, params.ContestLogin, params.Lang.Or("ru"))
	if err != nil {
		return corev1.DownloadContestStatementsPdfOK{}, err
	}
	return corev1.DownloadContestStatementsPdfOK{Data: bytes.NewReader(data)}, nil
}

func (h *CoreServer) DeleteContestProblem(ctx context.Context, params corev1.DeleteContestProblemParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.Contests.DeleteContestProblem(ctx, contestsmodel.ContestProblemDeletion{
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

	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	reorderItems := make([]contestsmodel.ContestProblemReorderItem, len(req.Problems))
	for i, item := range req.Problems {
		reorderItems[i] = contestsmodel.ContestProblemReorderItem{
			ProblemID: item.ProblemID,
			Position:  item.Position,
		}
	}

	err = h.Contests.ReorderContestProblems(ctx, contest.ID, reorderItems)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) CreateContestMember(ctx context.Context, params corev1.CreateContestMemberParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.Contests.CreateParticipant(ctx, contestsmodel.ParticipantCreation{
		ContestId: contest.ID,
		UserId:    params.UserID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContestMember(ctx context.Context, params corev1.DeleteContestMemberParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.Contests.DeleteParticipant(ctx, contestsmodel.ParticipantDeletion{
		ContestId: contest.ID,
		UserId:    params.UserID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateContestMember(ctx context.Context, params corev1.UpdateContestMemberParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	user := accessmodel.UserFromContext(ctx)

	// Prevent user from updating their own role
	if params.UserID == user.Id {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot update own role")
	}

	roleStr := string(params.Role)
	// Validate role value
	if roleStr != contestsmodel.ContestRoleOwner && roleStr != contestsmodel.ContestRoleModerator && roleStr != contestsmodel.ContestRoleParticipant {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	err = h.Contests.UpdateContestMember(ctx, contest.ID, params.UserID, roleStr)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListContestMembers(ctx context.Context, params corev1.ListContestMembersParams) (*corev1.ListContestMembersResponseModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)

	participantsList, err := h.Contests.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{
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
			ContestRole: corev1.ContestMemberModelContestRole(user.ContestRole),
			UserID:      user.UserID,
			Username:    user.Username,
			Role:        corev1.ContestMemberModelRole(user.Role),
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		}
	}

	return projectResponse(h, ctx, &resp, contest.ID)
}

func (h *CoreServer) GetMyContestRole(ctx context.Context, params corev1.GetMyContestRoleParams) (*corev1.GetMyContestRoleResponseModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	user := accessmodel.UserFromContext(ctx)

	contestRole, err := h.ContestPermissions.GetEffectiveContestRole(ctx, contest.ID, user.Id)
	if err != nil {
		return nil, err
	}

	if contestRole == nil {
		return projectResponse(h, ctx, &corev1.GetMyContestRoleResponseModel{}, contest.ID)
	}

	return projectResponse(h, ctx, &corev1.GetMyContestRoleResponseModel{
		Role: *contestRole,
	}, contest.ID)
}

func (h *CoreServer) ListContestSubmissions(ctx context.Context, params corev1.ListContestSubmissionsParams) (*corev1.ListSubmissionsResponseModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
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
		if params.SortOrder.Value == corev1.SubmissionSortOrderQueryDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	var state *submissionsmodel.State
	if params.State.IsSet() {
		stateVal := submissionsmodel.State(params.State.Value)
		state = &stateVal
	}

	var langName *submissionsmodel.LanguageName
	if params.Language.IsSet() {
		lang := submissionsmodel.LanguageName(params.Language.Value)
		langName = &lang
	}

	var problemID *uuid.UUID
	if params.ProblemId.IsSet() {
		problemID = &params.ProblemId.Value
	}

	var since int64
	if h.NATS != nil {
		lastSeq, seqErr := pkg.GetSubmissionsLastSequence(ctx, h.NATS)
		if seqErr != nil {
			slog.Warn("failed to get submissions last sequence", "error", seqErr)
		} else {
			since = safeInt64(lastSeq)
		}
	}

	submissionsList, err := h.Submissions.ListSubmissions(ctx, submissionsmodel.SubmissionsFilter{
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

	resp := SubmissionsListToDTO(submissionsList)
	if since != 0 {
		resp.Since = corev1.NewOptInt64(since)
	}

	return resp, nil
}

func (h *CoreServer) GetContestScoreboard(ctx context.Context, params corev1.GetContestScoreboardParams) (*corev1.ScoreboardResponseModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	unfrozen := params.Unfrozen.Or(false)

	var since int64
	if h.NATS != nil {
		lastSeq, seqErr := pkg.GetSubmissionsLastSequence(ctx, h.NATS)
		if seqErr != nil {
			return nil, seqErr
		}
		since = safeInt64(lastSeq)
	}

	sb, err := h.Contests.GetContestScoreboard(ctx, contest.ID, unfrozen)
	if err != nil {
		return nil, err
	}

	response := GetScoreboardResponseDTO(sb)
	response.Since = corev1.NewOptInt64(since)
	return response, nil
}

func (h *CoreServer) ListContestTeams(ctx context.Context, params corev1.ListContestTeamsParams) (*corev1.ListContestTeamsResponseModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	teams, err := h.Contests.GetContestTeams(ctx, contest.ID)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, listContestTeamsDTO(teams), contest.ID)
}

func (h *CoreServer) CreateContestTeam(ctx context.Context, params corev1.CreateContestTeamParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	role := contestsmodel.ContestRoleParticipant
	if params.Role.IsSet() && params.Role.Value != "" {
		role = contestsmodel.ContestRole(params.Role.Value)
	}

	err = h.Contests.CreateContestTeam(ctx, contest.ID, params.TeamID, role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateContestTeam(ctx context.Context, params corev1.UpdateContestTeamParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.Contests.UpdateContestTeamRole(ctx, contest.ID, params.TeamID, contestsmodel.ContestRole(params.Role))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContestTeam(ctx context.Context, params corev1.DeleteContestTeamParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.Contests.DeleteContestTeam(ctx, contest.ID, params.TeamID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) BlockProblemForUser(ctx context.Context, req corev1.OptBlockProblemRequestModel, params corev1.BlockProblemForUserParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	var reason *string
	if req.IsSet() && req.Value.Reason.IsSet() {
		reason = &req.Value.Reason.Value
	}

	err = h.Contests.BlockProblemForUser(ctx, contest.ID, params.UserID, params.ProblemID, reason)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UnblockProblemForUser(ctx context.Context, params corev1.UnblockProblemForUserParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	rejudgeSubmissions := params.RejudgeSubmissions.Or(false)

	err = h.Contests.UnblockProblemForUser(ctx, contest.ID, params.UserID, params.ProblemID, rejudgeSubmissions)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) GetProblemBlockStatusForUser(ctx context.Context, params corev1.GetProblemBlockStatusForUserParams) (*corev1.ProblemBlockStatusResponseModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	block, err := h.Contests.GetProblemBlockStatusForUser(ctx, contest.ID, params.UserID, params.ProblemID)
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

// ListContestJoinRequests handles GET /organizations/{org_login}/contests/{contest_login}/requests
func (h *CoreServer) ListContestJoinRequests(ctx context.Context, params corev1.ListContestJoinRequestsParams) (*corev1.ListContestJoinRequestsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	reqs, err := h.Contests.ListJoinRequests(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, wrapContestUCError(err, "failed to list contest join requests")
	}

	return ListContestJoinRequestsResponseDTO(reqs), nil
}

// CreateContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests
func (h *CoreServer) CreateContestJoinRequest(ctx context.Context, req corev1.OptCreateContestJoinRequestModel, params corev1.CreateContestJoinRequestParams) (*corev1.ContestJoinRequestResponseModel, error) {

	var message *string
	if req.IsSet() && req.Value.Message.IsSet() {
		message = &req.Value.Message.Value
	}

	joinReq, registered, err := h.Contests.CreateJoinRequest(ctx, params.OrgLogin, params.ContestLogin, message)
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
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return &corev1.ContestJoinRequestNullableResponseModel{}, nil
	}

	joinReq, err := h.Contests.GetMyPendingJoinRequest(ctx, params.OrgLogin, params.ContestLogin)
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

	err := h.Contests.CancelJoinRequest(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return wrapContestUCError(err, "failed to cancel contest join request")
	}

	return nil
}

// ApproveContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests/{id}/approve
func (h *CoreServer) ApproveContestJoinRequest(ctx context.Context, params corev1.ApproveContestJoinRequestParams) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.Contests.ApproveJoinRequest(ctx, params.OrgLogin, params.ContestLogin, params.ID)
	if err != nil {
		return wrapContestUCError(err, "failed to approve contest join request")
	}

	return nil
}

// RejectContestJoinRequest handles POST /organizations/{org_login}/contests/{contest_login}/requests/{id}/reject
func (h *CoreServer) RejectContestJoinRequest(ctx context.Context, params corev1.RejectContestJoinRequestParams) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.Contests.RejectJoinRequest(ctx, params.OrgLogin, params.ContestLogin, params.ID)
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
	// Adding context must not change the use case error classification.
	// Unclassified failures still map to HTTP 500 in pkg.ToREST.
	return pkg.Wrap(nil, err, fallbackMsg)
}
