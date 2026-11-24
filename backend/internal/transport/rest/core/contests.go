package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

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

// Candidate logins are tried in order; the unique index rejects taken ones.
func contestLoginCandidate(base string, suffix int) string {
	if suffix == 1 {
		return base
	}
	suffixStr := fmt.Sprintf("-%d", suffix)
	maxBaseLen := 64 - len(suffixStr)
	if len(base) > maxBaseLen {
		return strings.TrimRight(base[:maxBaseLen], "-") + suffixStr
	}
	return base + suffixStr
}

const maxContestLoginAttempts = 100

func (h *CoreServer) CreateContest(ctx context.Context, req *corev1.CreateContestRequestModel) (*corev1.CreationResponseModel, error) {
	if err := validateCreateContestParams(req.Title, req.Login); err != nil {
		return nil, err
	}
	input := &contestsmodel.CreateContestInput{
		OrganizationID: req.OrganizationID,
		Title:          req.Title,
		Visibility:     contestsmodel.ContestVisibilityPrivate,
	}
	if req.Login.Value != "" {
		input.Login = strings.ToLower(req.Login.Value)
		id, err := h.Contests.CreateContest(ctx, input)
		if err != nil {
			return nil, err
		}
		return &corev1.CreationResponseModel{ID: id, Login: corev1.NewOptString(input.Login)}, nil
	}
	base := slugifyContestTitle(req.Title)
	for suffix := 1; ; suffix++ {
		input.Login = contestLoginCandidate(base, suffix)
		if isReservedContestLogin(input.Login) {
			continue
		}
		id, err := h.Contests.CreateContest(ctx, input)
		if errors.Is(err, pkg.ErrContestLoginTaken) && suffix < maxContestLoginAttempts {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &corev1.CreationResponseModel{ID: id, Login: corev1.NewOptString(input.Login)}, nil
	}
}

func (h *CoreServer) ListContests(ctx context.Context, params corev1.ListContestsParams) (*corev1.ListContestsResponseModel, error) {
	filter := contestsmodel.ListContestsFilter{
		Page:     params.Page.Or(1),
		PageSize: params.PageSize.Or(50),
		Search:   params.Search.Or(""),
	}
	if v, ok := params.OrganizationID.Get(); ok {
		filter.OrganizationID = &v
	}
	if v, ok := params.TeamID.Get(); ok {
		filter.TeamID = &v
	}
	if v, ok := params.Visibility.Get(); ok {
		filter.Visibility = string(v)
	}
	if v, ok := params.SortBy.Get(); ok {
		filter.SortBy = string(v)
	}
	if v, ok := params.SortOrder.Get(); ok {
		filter.SortOrder = string(v)
	}
	list, err := h.Contests.ListContests(ctx, filter)
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
	response := GetContestResponseDTO(details.Contest, details.Problems)
	if details.Role != "" {
		response.MyRole = corev1.NewOptString(string(details.Role))
	}
	if details.JoinRequest != nil {
		response.MyJoinRequest = corev1.NewOptContestJoinRequestModel(ContestJoinRequestDTO(*details.JoinRequest))
	}
	if details.VirtualRun != nil {
		response.VirtualRun = corev1.NewOptVirtualRunModel(virtualRunDTO(*details.VirtualRun))
	}
	return projectResponse(h, ctx, response)
}

func (h *CoreServer) UpdateContest(ctx context.Context, req *corev1.UpdateContestRequestModel, params corev1.UpdateContestParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrInvalid, nil, "missing request body")
	}

	err := validateUpdateContestRequest(req)
	if err != nil {
		return err
	}

	var newLogin *string
	if req.Login.Value != "" {
		cleaned := strings.ToLower(req.Login.Value)
		newLogin = &cleaned
	}

	settings := contestsmodel.ContestSettingsPatch{
		ScoreboardDuring:    optScope(req.ScoreboardDuring),
		ScoreboardAfter:     optScope(req.ScoreboardAfter),
		SubmissionsDuring:   optScope(req.SubmissionsDuring),
		SubmissionsAfter:    optScope(req.SubmissionsAfter),
		SourcesDuring:       optScope(req.SourcesDuring),
		SourcesAfter:        optScope(req.SourcesAfter),
		ReportsDuring:       optScope(req.ReportsDuring),
		ReportsAfter:        optScope(req.ReportsAfter),
		FeedbackDuring:      optFeedback(req.FeedbackDuring),
		FeedbackAfter:       optFeedback(req.FeedbackAfter),
		FreezeHidesAttempts: optBool(req.FreezeHidesAttempts),
		UnfreezeTimeSet:     req.UnfreezeTime.IsSet(),
		FreezeDurationSet:   req.FreezeDurationMinutes.IsSet(),
		HideStatements:      optBool(req.HideStatements),
		AllowClarifications: optBool(req.AllowClarifications),
		EnableDrafts:        optBool(req.EnableDrafts),
		EnableUpsolving:     optBool(req.EnableUpsolving),
		AllowVirtual:        optBool(req.AllowVirtual),
		Pretests:            optBool(req.Pretests),
	}
	if v, ok := req.FreezeDurationMinutes.Get(); ok {
		settings.FreezeDurationMinutes = &v
	}
	if v, ok := req.UnfreezeTime.Get(); ok {
		settings.UnfreezeTime = &v
	}
	if v, ok := req.FreezeStatus.Get(); ok {
		status := contestsmodel.FreezeStatus(v)
		settings.FreezeStatus = &status
	}
	if v, ok := req.JoinPolicy.Get(); ok {
		policy := accessmodel.JoinPolicy(v)
		settings.JoinPolicy = &policy
	}
	if req.Languages != nil {
		settings.Languages = []string{}
		for _, id := range req.Languages {
			if _, ok := h.languages.Langs[id]; !ok {
				return pkg.Invalid(pkg.ErrUnknownLanguage, "unknown language configuration "+id, map[string]any{"id": id})
			}
			if !slices.Contains(settings.Languages, id) {
				settings.Languages = append(settings.Languages, id)
			}
		}
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
		ID:           params.ContestID,
		Login:        newLogin,
		Title:        reqTitle,
		Description:  reqDesc,
		Visibility:   reqVisibility,
		Settings:     settings,
		StartTime:    startTime,
		StartTimeSet: req.StartTime.IsSet(),
		EndTime:      endTime,
		EndTimeSet:   req.EndTime.IsSet(),
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContest(ctx context.Context, params corev1.DeleteContestParams) error {
	err := h.Contests.DeleteContest(ctx, params.ContestID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) CreateContestProblem(ctx context.Context, params corev1.CreateContestProblemParams) error {
	pkgID := uuid.Nil
	if params.PackageID.IsSet() {
		pkgID = params.PackageID.Value
	}
	err := h.Contests.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{
		ContestId: params.ContestID,
		ProblemId: params.ProblemID,
		PackageId: pkgID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) GetContestProblem(ctx context.Context, params corev1.GetContestProblemParams) (*corev1.GetContestProblemResponseModel, error) {
	details, err := h.ContestDetails.GetContestProblemDetails(ctx, params.ContestID, params.ProblemID)
	if err != nil {
		return nil, err
	}
	response := GetContestProblemResponseDTO(details.Link, details.Problem, details.Statement, SamplesDTO(details.Samples))
	response.Problem.Assets = details.Assets
	return projectResponse(h, ctx, response, details.Link.ContestID)
}

// Prevent directly opened statement assets, especially SVG, from running scripts in the site origin.
const statementAssetPolicy = "default-src 'none'; style-src 'unsafe-inline'; sandbox"

func (h *CoreServer) GetContestProblemAsset(ctx context.Context, params corev1.GetContestProblemAssetParams) (*corev1.GetContestProblemAssetOKHeaders, error) {
	file, contentType, err := h.ContestDetails.OpenProblemAsset(ctx, params.ContestID, params.ProblemID, params.SHA256)
	if err != nil {
		return nil, err
	}
	return &corev1.GetContestProblemAssetOKHeaders{
		// Only private caching is safe: shared caches could serve assets after access is revoked.
		CacheControl:          corev1.NewOptString("private, max-age=31536000, immutable"),
		ETag:                  corev1.NewOptString(`"` + params.SHA256 + `"`),
		ContentSecurityPolicy: corev1.NewOptString(statementAssetPolicy),
		ContentType:           contentType,
		Response:              corev1.GetContestProblemAssetOK{Data: file},
	}, nil
}

func (h *CoreServer) DownloadContestStatementsPdf(ctx context.Context, params corev1.DownloadContestStatementsPdfParams) (corev1.DownloadContestStatementsPdfOK, error) {
	data, err := h.ContestDetails.ReadBooklet(ctx, params.ContestID, params.Lang.Or("ru"))
	if err != nil {
		return corev1.DownloadContestStatementsPdfOK{}, err
	}
	return corev1.DownloadContestStatementsPdfOK{Data: bytes.NewReader(data)}, nil
}

func (h *CoreServer) DownloadContestProblemStatementPdf(ctx context.Context, params corev1.DownloadContestProblemStatementPdfParams) (corev1.DownloadContestProblemStatementPdfOK, error) {
	file, err := h.ContestDetails.OpenProblemPDF(ctx, params.ContestID, params.ProblemID, params.Lang.Or(""))
	if err != nil {
		return corev1.DownloadContestProblemStatementPdfOK{}, err
	}
	return corev1.DownloadContestProblemStatementPdfOK{Data: file}, nil
}

func (h *CoreServer) DeleteContestProblem(ctx context.Context, params corev1.DeleteContestProblemParams) error {
	err := h.Contests.DeleteContestProblem(ctx, contestsmodel.ContestProblemDeletion{
		ContestId: params.ContestID,
		ProblemId: params.ProblemID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ReorderContestProblems(ctx context.Context, req *corev1.ReorderContestProblemsRequestModel, params corev1.ReorderContestProblemsParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrInvalid, nil, "missing request body")
	}

	reorderItems := make([]contestsmodel.ContestProblemReorderItem, len(req.Problems))
	for i, item := range req.Problems {
		reorderItems[i] = contestsmodel.ContestProblemReorderItem{
			ProblemID: item.ProblemID,
			Position:  item.Position,
		}
	}

	err := h.Contests.ReorderContestProblems(ctx, params.ContestID, reorderItems)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContestMember(ctx context.Context, params corev1.DeleteContestMemberParams) error {
	err := h.Contests.DeleteParticipant(ctx, contestsmodel.ParticipantDeletion{
		ContestId: params.ContestID,
		UserId:    params.UserID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateContestMember(ctx context.Context, params corev1.UpdateContestMemberParams) error {
	return h.Contests.UpdateContestMember(ctx, params.ContestID, params.UserID, accessmodel.ContestRole(params.Role))
}

func (h *CoreServer) ListContestMembers(ctx context.Context, params corev1.ListContestMembersParams) (*corev1.ListContestMembersResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)

	participantsList, err := h.Contests.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{
		Page:      page,
		PageSize:  pageSize,
		ContestId: params.ContestID,
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
			ContestID:   params.ContestID,
			ContestRole: corev1.ContestMemberModelContestRole(user.ContestRole),
			UserID:      user.UserID,
			Username:    user.Username,
			Role:        corev1.ContestMemberModelRole(user.Role),
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		}
	}

	return projectResponse(h, ctx, &resp, params.ContestID)
}

func (h *CoreServer) ListContestSubmissions(ctx context.Context, params corev1.ListContestSubmissionsParams) (*corev1.ListSubmissionsResponseModel, error) {
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

	var langName *string
	if params.Language.IsSet() {
		langName = &params.Language.Value
	}

	var problemID *uuid.UUID
	if params.ProblemId.IsSet() {
		problemID = &params.ProblemId.Value
	}

	submissionsList, err := h.Submissions.ListSubmissions(ctx, submissionsmodel.SubmissionsFilter{
		ContestId: &params.ContestID,
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

	return SubmissionsListToDTO(submissionsList), nil
}

func (h *CoreServer) GetContestScoreboard(ctx context.Context, params corev1.GetContestScoreboardParams) (*corev1.ScoreboardResponseModel, error) {
	unfrozen := params.Unfrozen.Or(false)

	sb, err := h.Contests.GetContestScoreboard(ctx, params.ContestID, unfrozen, params.Virtual.Or(false))
	if err != nil {
		return nil, err
	}

	return GetScoreboardResponseDTO(sb), nil
}

func (h *CoreServer) ListContestTeams(ctx context.Context, params corev1.ListContestTeamsParams) (*corev1.ListContestTeamsResponseModel, error) {
	teams, err := h.Contests.GetContestTeams(ctx, params.ContestID)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, listContestTeamsDTO(teams), params.ContestID)
}

func (h *CoreServer) CreateContestTeam(ctx context.Context, params corev1.CreateContestTeamParams) error {
	role := accessmodel.ContestParticipant
	if params.Role.IsSet() && params.Role.Value != "" {
		role = accessmodel.ContestRole(params.Role.Value)
	}

	err := h.Contests.CreateContestTeam(ctx, params.ContestID, params.TeamID, role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateContestTeam(ctx context.Context, params corev1.UpdateContestTeamParams) error {
	err := h.Contests.UpdateContestTeamRole(ctx, params.ContestID, params.TeamID, accessmodel.ContestRole(params.Role))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteContestTeam(ctx context.Context, params corev1.DeleteContestTeamParams) error {
	err := h.Contests.DeleteContestTeam(ctx, params.ContestID, params.TeamID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) BlockProblemForUser(ctx context.Context, req corev1.OptBlockProblemRequestModel, params corev1.BlockProblemForUserParams) error {
	var reason *string
	if req.IsSet() && req.Value.Reason.IsSet() {
		reason = &req.Value.Reason.Value
	}

	err := h.Contests.BlockProblemForUser(ctx, params.ContestID, params.UserID, params.ProblemID, reason)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UnblockProblemForUser(ctx context.Context, params corev1.UnblockProblemForUserParams) error {
	rejudgeSubmissions := params.RejudgeSubmissions.Or(false)

	err := h.Contests.UnblockProblemForUser(ctx, params.ContestID, params.UserID, params.ProblemID, rejudgeSubmissions)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) GetProblemBlockStatusForUser(ctx context.Context, params corev1.GetProblemBlockStatusForUserParams) (*corev1.ProblemBlockStatusResponseModel, error) {
	block, err := h.Contests.GetProblemBlockStatusForUser(ctx, params.ContestID, params.UserID, params.ProblemID)
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

func (h *CoreServer) ListContestJoinRequests(ctx context.Context, params corev1.ListContestJoinRequestsParams) (*corev1.ListContestJoinRequestsResponseModel, error) {
	reqs, err := h.Contests.ListJoinRequests(ctx, params.ContestID)
	if err != nil {
		return nil, err
	}

	return ListContestJoinRequestsResponseDTO(reqs), nil
}

func (h *CoreServer) CreateContestJoinRequest(ctx context.Context, req corev1.OptCreateContestJoinRequestModel, params corev1.CreateContestJoinRequestParams) (*corev1.ContestJoinRequestResponseModel, error) {

	var message *string
	if req.IsSet() && req.Value.Message.IsSet() {
		message = &req.Value.Message.Value
	}

	joinReq, registered, err := h.Contests.CreateJoinRequest(ctx, params.ContestID, message)
	if err != nil {
		return nil, err
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

func (h *CoreServer) CancelContestJoinRequest(ctx context.Context, params corev1.CancelContestJoinRequestParams) error {
	return h.Contests.CancelJoinRequest(ctx, params.ContestID, params.ID)
}

func (h *CoreServer) ResolveContestJoinRequest(ctx context.Context, req *corev1.ResolveJoinRequestModel, params corev1.ResolveContestJoinRequestParams) error {
	return h.Contests.ResolveJoinRequest(ctx, params.ContestID, params.ID, req.Status == corev1.ResolveJoinRequestModelStatusApproved)
}

func optScope(v corev1.OptContestScope) *contestsmodel.ContestScope {
	if !v.IsSet() {
		return nil
	}
	scope := contestsmodel.ContestScope(v.Value)
	return &scope
}

func optFeedback(v corev1.OptFeedbackLevel) *contestsmodel.FeedbackLevel {
	if !v.IsSet() {
		return nil
	}
	level := contestsmodel.FeedbackLevel(v.Value)
	return &level
}

func optBool(v corev1.OptBool) *bool {
	if !v.IsSet() {
		return nil
	}
	return &v.Value
}

func virtualRunDTO(run contestsmodel.VirtualRun) corev1.VirtualRunModel {
	return corev1.VirtualRunModel{StartedAt: run.StartedAt, EndsAt: run.Over()}
}

func (h *CoreServer) StartVirtualRun(ctx context.Context, params corev1.StartVirtualRunParams) (*corev1.VirtualRunModel, error) {
	run, err := h.Contests.StartVirtualRun(ctx, params.ContestID)
	if err != nil {
		return nil, err
	}
	dto := virtualRunDTO(run)
	return &dto, nil
}

func (h *CoreServer) FinishVirtualRun(ctx context.Context, _ *corev1.FinishVirtualRunRequestModel, params corev1.FinishVirtualRunParams) error {
	return h.Contests.FinishVirtualRun(ctx, params.ContestID)
}
