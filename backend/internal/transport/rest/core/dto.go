package core

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/go-faster/jx"
	"github.com/google/uuid"
)

func safeInt32[T ~int | ~int64](v T) int32 {
	if int64(v) > math.MaxInt32 {
		return math.MaxInt32
	}
	if int64(v) < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

func timePtrToOptNilDateTime(t *time.Time) corev1.OptNilDateTime {
	if t == nil {
		return corev1.OptNilDateTime{}
	}
	return corev1.NewOptNilDateTime(*t)
}

func timePtrToOptDateTime(t *time.Time) corev1.OptDateTime {
	if t == nil {
		return corev1.OptDateTime{}
	}
	return corev1.NewOptDateTime(*t)
}

func stringPtrToOptNilString(s *string) corev1.OptNilString {
	if s == nil {
		return corev1.OptNilString{}
	}
	return corev1.NewOptNilString(*s)
}

func stringPtrToOptString(s *string) corev1.OptString {
	if s == nil {
		return corev1.OptString{}
	}
	return corev1.NewOptString(*s)
}

func stringToOptString(s string) corev1.OptString {
	if s == "" {
		return corev1.OptString{}
	}
	return corev1.NewOptString(s)
}

func uuidPtrToOptUUID(u *uuid.UUID) corev1.OptUUID {
	if u == nil || *u == uuid.Nil {
		return corev1.OptUUID{}
	}
	return corev1.NewOptUUID(*u)
}

func int32PtrToOptNilInt32(i *int32) corev1.OptNilInt32 {
	if i == nil {
		return corev1.OptNilInt32{}
	}
	return corev1.NewOptNilInt32(*i)
}

func PaginationDTO(p paging.Pagination) corev1.PaginationModel {
	return corev1.PaginationModel{
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalItems: p.TotalItems,
		TotalPages: p.TotalPages,
	}
}

func uuidPtrToUUID(ptr *uuid.UUID) uuid.UUID {
	if ptr == nil {
		return uuid.Nil
	}
	return *ptr
}

func int32PtrToInt32(ptr *int32) int32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func GetContestResponseDTO(contest contestsmodel.Contest, problems []contestsmodel.ContestProblem) *corev1.GetContestResponseModel {
	resp := &corev1.GetContestResponseModel{Contest: ContestDTO(contest), Problems: make([]corev1.ContestProblemListItemModel, len(problems))}
	for i, p := range problems {
		resp.Problems[i] = ContestProblemsListItemDTO(p)
	}
	return resp
}

func ListContestsResponseDTO(contestsList *contestsmodel.ContestsList) *corev1.ListContestsResponseModel {
	resp := corev1.ListContestsResponseModel{
		Contests:   make([]corev1.ContestModel, len(contestsList.Contests)),
		Pagination: PaginationDTO(contestsList.Pagination),
	}

	for i, contest := range contestsList.Contests {
		resp.Contests[i] = ContestDTO(contest)
	}

	return &resp
}

func GetContestProblemResponseDTO(contestProblem contestsmodel.ContestProblem, problem problemsmodel.Problem, statement *problemsmodel.Statement, samples []corev1.ProblemSampleModel) *corev1.GetContestProblemResponseModel {
	title := strings.TrimSpace(problem.Title)
	if title == "" {
		title = strings.TrimSpace(contestProblem.Title)
	}
	if statement != nil {
		statementTitle := strings.TrimSpace(statement.Title)
		if statementTitle != "" {
			title = statementTitle
		}
	}

	if samples == nil {
		samples = []corev1.ProblemSampleModel{}
	}

	return &corev1.GetContestProblemResponseModel{
		Problem: corev1.ContestProblemModel{
			ProblemID:    contestProblem.ProblemID,
			Title:        title,
			TimeLimit:    safeInt32(problem.TimeLimitMs),
			MemoryLimit:  safeInt32(problem.MemoryLimitMb),
			InputFile:    contestProblem.InputFile,
			OutputFile:   contestProblem.OutputFile,
			Position:     safeInt32(contestProblem.Ordinal),
			Legend:       statementField(statement, func(s problemsmodel.Statement) string { return s.Legend }),
			InputFormat:  statementField(statement, func(s problemsmodel.Statement) string { return s.InputFormat }),
			OutputFormat: statementField(statement, func(s problemsmodel.Statement) string { return s.OutputFormat }),
			Notes:        statementField(statement, func(s problemsmodel.Statement) string { return s.Notes }),
			Interaction:  statementField(statement, func(s problemsmodel.Statement) string { return s.Interaction }),
			Scoring:      statementField(statement, func(s problemsmodel.Statement) string { return s.Scoring }),
			CreatedAt:    problem.CreatedAt,
			UpdatedAt:    problem.UpdatedAt,
			Samples:      samples,
			Assets:       map[string]string{},
		},
	}
}

func SubmissionsListToDTO(submissionsList *submissionsmodel.SubmissionsList) *corev1.ListSubmissionsResponseModel {
	resp := corev1.ListSubmissionsResponseModel{
		TotalCount:  submissionsList.TotalCount,
		Submissions: make([]corev1.SubmissionsListItemModel, len(submissionsList.Submissions)),
		Pagination:  PaginationDTO(submissionsList.Pagination),
	}

	for i, solution := range submissionsList.Submissions {
		resp.Submissions[i] = SubmissionListItemDTO(solution)
	}

	return &resp
}

func ContestDTO(c contestsmodel.Contest) corev1.ContestModel {
	title := c.Title

	visibility := string(c.Visibility)
	s := c.Settings
	model := corev1.ContestModel{
		ID:                    c.ID,
		Login:                 c.Login,
		OrganizationID:        c.OrganizationID,
		OrganizationLogin:     c.OrganizationLogin,
		Title:                 title,
		Description:           c.Description,
		Visibility:            visibility,
		ScoreboardDuring:      corev1.ContestScope(s.Scoreboard.During),
		ScoreboardAfter:       corev1.ContestScope(s.Scoreboard.After),
		SubmissionsDuring:     corev1.ContestScope(s.Submissions.During),
		SubmissionsAfter:      corev1.ContestScope(s.Submissions.After),
		SourcesDuring:         corev1.ContestScope(s.Sources.During),
		SourcesAfter:          corev1.ContestScope(s.Sources.After),
		ReportsDuring:         corev1.ContestScope(s.Reports.During),
		ReportsAfter:          corev1.ContestScope(s.Reports.After),
		FeedbackDuring:        corev1.FeedbackLevel(s.Feedback.During),
		FeedbackAfter:         corev1.FeedbackLevel(s.Feedback.After),
		FreezeHidesAttempts:   s.FreezeHidesAttempts,
		UnfreezeTime:          timePtrToOptNilDateTime(s.UnfreezeTime),
		FreezeDurationMinutes: int32PtrToOptNilInt32(s.FreezeDurationMinutes),
		FreezeStatus:          corev1.ContestModelFreezeStatus(s.FreezeStatus),
		EnableDrafts:          s.EnableDrafts,
		AllowClarifications:   s.AllowClarifications,
		EnableUpsolving:       s.EnableUpsolving,
		AllowVirtual:          s.AllowVirtual,
		Pretests:              s.Pretests,
		JoinPolicy:            corev1.ContestModelJoinPolicy(s.JoinPolicy),
		HideStatements:        s.HideStatements,
		Languages:             append([]string{}, s.Languages...),
		CreatedAt:             c.CreatedAt,
		UpdatedAt:             c.UpdatedAt,
		StartTime:             timePtrToOptNilDateTime(c.StartTime),
		EndTime:               timePtrToOptNilDateTime(c.EndTime),
	}

	return model
}

func ContestProblemsListItemDTO(t contestsmodel.ContestProblem) corev1.ContestProblemListItemModel {
	return corev1.ContestProblemListItemModel{ProblemID: t.ProblemID, Position: safeInt32(t.Ordinal), Title: strings.TrimSpace(t.Title), MemoryLimit: safeInt32(t.MemoryLimitMb), TimeLimit: safeInt32(t.TimeLimitMs), CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}
func SamplesDTO(samples []problemsmodel.Sample) []corev1.ProblemSampleModel {
	result := make([]corev1.ProblemSampleModel, len(samples))
	for i, s := range samples {
		result[i] = corev1.ProblemSampleModel{Input: s.Input, Output: s.Output}
	}
	return result
}

func UserDTO(u accessmodel.User) corev1.UserModel {
	return userDTO(u)
}

func ParticipantDTO(p contestsmodel.ContestMember) corev1.UserModel {
	return corev1.UserModel{
		ID:        p.UserID,
		Username:  p.Username,
		Role:      string(p.Role),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func ProblemsListItemDTO(p problemsmodel.Problem) corev1.ProblemsListItemModel {
	title := p.Title

	return corev1.ProblemsListItemModel{
		ID:                p.ID,
		OrganizationID:    p.OrganizationID,
		OrganizationLogin: p.OrganizationLogin,
		Title:             title,
		Visibility:        stringToOptString(p.Visibility),
		MemoryLimit:       safeInt32(p.MemoryLimitMb),
		TimeLimit:         safeInt32(p.TimeLimitMs),
		IsTemplate:        p.IsTemplate,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}

func ListProblemsResponseDTO(problemsList *problemsmodel.ProblemsList) *corev1.ListProblemsResponseModel {
	resp := corev1.ListProblemsResponseModel{
		Problems:   make([]corev1.ProblemsListItemModel, len(problemsList.Problems)),
		Pagination: PaginationDTO(problemsList.Pagination),
	}

	for i, problem := range problemsList.Problems {
		resp.Problems[i] = ProblemsListItemDTO(problem)
	}

	return &resp
}

func ProblemDTO(p problemsmodel.Problem) *corev1.ProblemModel {
	return &corev1.ProblemModel{
		ID:                p.ID,
		OrganizationID:    p.OrganizationID,
		OrganizationLogin: p.OrganizationLogin,
		Title:             strings.TrimSpace(p.Title),
		Visibility:        p.Visibility,
		TimeLimit:         safeInt32(p.TimeLimitMs),
		MemoryLimit:       safeInt32(p.MemoryLimitMb),
		IsTemplate:        p.IsTemplate,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}

func statementField(statement *problemsmodel.Statement, getter func(problemsmodel.Statement) string) string {
	if statement == nil {
		return ""
	}

	return getter(*statement)
}

func SubmissionListItemDTO(s submissionsmodel.Submission) corev1.SubmissionsListItemModel {
	dto := corev1.SubmissionsListItemModel{
		Revision: s.Revision,
		ID:       s.ID,
		Kind:     corev1.SubmissionKind(s.Kind),
		UserID:   uuidPtrToUUID(s.CreatedBy),

		Username: s.Username,

		State:      corev1.NewOptInt32(s.State),
		Score:      corev1.NewOptFloat64(s.Score.Float64()),
		Penalty:    corev1.NewOptInt32(s.Penalty),
		TimeStat:   corev1.NewOptInt32(s.TimeStat),
		MemoryStat: corev1.NewOptInt32(s.MemoryStat),
		Language:   s.Language,
		FailedTest: int32PtrToOptNilInt32(s.FailedTest),
		Pretests:   corev1.NewOptBool(s.Pretests),

		ProblemID:    uuidPtrToUUID(s.ProblemID),
		ProblemTitle: s.ProblemTitle,

		Position: int32PtrToInt32(s.Position),

		ContestID:         uuidPtrToUUID(s.ContestID),
		ContestLogin:      s.ContestLogin,
		ContestTitle:      s.ContestTitle,
		OrganizationLogin: stringToOptString(s.OrganizationLogin),
		BanReason:         stringPtrToOptNilString(s.BanReason),

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.ResultHidden {
		dto.ResultHidden = corev1.NewOptBool(true)
		dto.State.Reset()
		dto.Score.Reset()
		dto.Penalty.Reset()
		dto.TimeStat.Reset()
		dto.MemoryStat.Reset()
		dto.FailedTest.Reset()
		dto.BanReason.Reset()
	}
	return dto

}

func SubmissionTestDetailsDTO(td *submissionsmodel.SubmissionTestDetails) *corev1.SubmissionTestDetailsModel {
	if td == nil {
		return nil
	}

	var tests []corev1.TestDetailItemModel
	if td.Tests != nil {
		tests = make([]corev1.TestDetailItemModel, len(td.Tests))
		for i, t := range td.Tests {
			tests[i] = corev1.TestDetailItemModel{
				TestIndex: t.TestIndex,
				Verdict:   t.Verdict,
				TimeMs:    t.TimeMs,
				MemoryKB:  t.MemoryKb,
				Sample:    corev1.NewOptBool(t.Sample),
				Subtask:   corev1.NewOptString(t.Subtask),
			}
		}
	}
	subtasks := make([]corev1.SubtaskResultModel, len(td.Subtasks))
	for i, s := range td.Subtasks {
		subtasks[i] = corev1.SubtaskResultModel{Name: s.Name}
		if s.Score != nil && s.Points != nil {
			subtasks[i].Score = corev1.NewOptFloat64(s.Score.Float64())
			subtasks[i].Points = corev1.NewOptFloat64(s.Points.Float64())
		}
	}

	var failedTestDetails corev1.OptFailedTestDetailModel
	if td.FailedTestDetails != nil {
		failedTestDetails = corev1.NewOptFailedTestDetailModel(corev1.FailedTestDetailModel{
			TestIndex:     td.FailedTestDetails.TestIndex,
			Input:         td.FailedTestDetails.Input,
			Output:        td.FailedTestDetails.Output,
			Answer:        td.FailedTestDetails.Answer,
			CheckerOutput: td.FailedTestDetails.CheckerOutput,
			ErrorMessage:  td.FailedTestDetails.ErrorMessage,
			IsTruncated:   td.FailedTestDetails.IsTruncated,
		})
	}

	return &corev1.SubmissionTestDetailsModel{
		CompilerOutput:    stringPtrToOptNilString(td.CompilerOutput),
		Tests:             tests,
		Subtasks:          subtasks,
		FailedTestDetails: failedTestDetails,
	}
}

func SolutionDTO(s submissionsmodel.Submission) corev1.SubmissionModel {
	var testDetailsOpt corev1.OptSubmissionTestDetailsModel
	if td := SubmissionTestDetailsDTO(s.TestDetails); td != nil {
		testDetailsOpt = corev1.NewOptSubmissionTestDetailsModel(*td)
	}

	dto := corev1.SubmissionModel{
		ID:     s.ID,
		Kind:   corev1.SubmissionKind(s.Kind),
		UserID: uuidPtrToUUID(s.CreatedBy),

		Username: s.Username,

		Submission: corev1.NewOptString(s.Submission),

		State:       corev1.NewOptInt32(s.State),
		Score:       corev1.NewOptFloat64(s.Score.Float64()),
		Penalty:     corev1.NewOptInt32(s.Penalty),
		TimeStat:    corev1.NewOptInt32(s.TimeStat),
		MemoryStat:  corev1.NewOptInt32(s.MemoryStat),
		Language:    s.Language,
		FailedTest:  int32PtrToOptNilInt32(s.FailedTest),
		TestDetails: testDetailsOpt,
		Pretests:    corev1.NewOptBool(s.Pretests),

		ProblemID:    uuidPtrToUUID(s.ProblemID),
		ProblemTitle: s.ProblemTitle,

		Position: int32PtrToInt32(s.Position),

		ContestID:         uuidPtrToUUID(s.ContestID),
		ContestLogin:      s.ContestLogin,
		ContestTitle:      s.ContestTitle,
		OrganizationLogin: stringToOptString(s.OrganizationLogin),
		BanReason:         stringPtrToOptNilString(s.BanReason),

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.ResultHidden {
		dto.ResultHidden = corev1.NewOptBool(true)
		dto.State.Reset()
		dto.Score.Reset()
		dto.Penalty.Reset()
		dto.TimeStat.Reset()
		dto.MemoryStat.Reset()
		dto.FailedTest.Reset()
		dto.BanReason.Reset()
	}
	if s.SourceHidden {
		dto.Submission.Reset()
	}
	return dto

}

func userDTO(u accessmodel.User) corev1.UserModel {
	return userProfileDTO(identitymodel.UserProfile{Id: u.Id, Username: u.Username, Role: u.Role, Email: u.Email, AvatarUrl: u.AvatarUrl, ExpiresAt: u.ExpiresAt, IsEmailVerified: u.IsEmailVerified, ClaimedByUserID: u.ClaimedByUserID, ClaimedAt: u.ClaimedAt, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt})
}

func userProfileDTO(u identitymodel.UserProfile) corev1.UserModel {
	var imgID corev1.OptUUID
	if u.AvatarUrl != nil && *u.AvatarUrl != "" {
		if parsed, err := uuid.Parse(*u.AvatarUrl); err == nil {
			imgID = corev1.NewOptUUID(parsed)
		}
	}

	return corev1.UserModel{
		ID:              u.Id,
		Username:        u.Username,
		Role:            string(u.Role),
		Email:           stringPtrToOptString(u.Email),
		ImgID:           imgID,
		ExpiresAt:       timePtrToOptDateTime(u.ExpiresAt),
		ClaimedByUserID: uuidPtrToOptUUID(u.ClaimedByUserID),
		ClaimedAt:       timePtrToOptDateTime(u.ClaimedAt),
		IsEmailVerified: corev1.NewOptBool(u.IsEmailVerified),
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}

func organizationDTO(o organizationsmodel.Organization) corev1.OrganizationModel {
	joinPolicy := corev1.OrganizationModelJoinPolicyByRequest
	if o.JoinPolicy != "" {
		joinPolicy = corev1.OrganizationModelJoinPolicy(o.JoinPolicy)
	}

	return corev1.OrganizationModel{
		ID:          o.ID,
		Login:       o.Login,
		Name:        o.Name,
		Description: stringToOptString(o.Description),
		JoinPolicy:  joinPolicy,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}

func listOrganizationsDTO(ol *organizationsmodel.OrganizationList) *corev1.ListOrganizationsResponseModel {
	resp := corev1.ListOrganizationsResponseModel{
		Organizations: make([]corev1.OrganizationModel, len(ol.Organizations)),
		Pagination:    PaginationDTO(ol.Pagination),
	}

	for i, org := range ol.Organizations {
		resp.Organizations[i] = organizationDTO(org)
	}

	return &resp
}

func organizationMemberDTO(m organizationsmodel.OrganizationMember) corev1.OrganizationMemberModel {
	return corev1.OrganizationMemberModel{
		UserID:         m.UserID,
		OrganizationID: m.OrganizationID,
		Username:       m.Username,
		Role:           corev1.OrganizationMemberModelRole(m.Role),
		CreatedAt:      m.CreatedAt,
	}
}

func listOrganizationMembersDTO(members []organizationsmodel.OrganizationMember, pagination paging.Pagination) *corev1.ListOrganizationMembersResponseModel {
	resp := corev1.ListOrganizationMembersResponseModel{
		Members:    make([]corev1.OrganizationMemberModel, len(members)),
		Pagination: PaginationDTO(pagination),
	}

	for i, member := range members {
		resp.Members[i] = organizationMemberDTO(member)
	}

	return &resp
}

func teamDTO(t organizationsmodel.Team) corev1.TeamModel {
	return corev1.TeamModel{
		ID:             t.ID,
		Name:           t.Name,
		OrganizationID: t.OrganizationID,
		Description:    stringToOptString(t.Description),
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

func listTeamsDTO(teams []organizationsmodel.Team, pagination paging.Pagination) *corev1.ListTeamsResponseModel {
	resp := corev1.ListTeamsResponseModel{
		Teams:      make([]corev1.TeamModel, len(teams)),
		Pagination: PaginationDTO(pagination),
	}

	for i, team := range teams {
		resp.Teams[i] = teamDTO(team)
	}

	return &resp
}

func teamMemberDTO(m organizationsmodel.TeamMember) corev1.TeamMemberModel {
	return corev1.TeamMemberModel{
		UserID:    m.UserID,
		TeamID:    m.TeamID,
		Username:  m.Username,
		Role:      corev1.TeamMemberModelRole(m.Role),
		CreatedAt: m.CreatedAt,
	}
}

func listTeamMembersDTO(members []organizationsmodel.TeamMember, pagination paging.Pagination) *corev1.ListTeamMembersResponseModel {
	resp := corev1.ListTeamMembersResponseModel{
		Members:    make([]corev1.TeamMemberModel, len(members)),
		Pagination: PaginationDTO(pagination),
	}

	for i, member := range members {
		resp.Members[i] = teamMemberDTO(member)
	}

	return &resp
}

func GetScoreboardResponseDTO(sb *contestsmodel.ScoreboardResponse) *corev1.ScoreboardResponseModel {
	if sb == nil {
		return &corev1.ScoreboardResponseModel{}
	}

	problems := make([]corev1.ScoreboardProblemHeaderModel, len(sb.Problems))
	for i, p := range sb.Problems {
		problems[i] = corev1.ScoreboardProblemHeaderModel{
			ProblemID: p.ProblemID,
			Title:     p.Title,
			ShortName: p.ShortName,
			Ordinal:   p.Ordinal,
		}
	}

	items := make([]corev1.ScoreboardItemModel, len(sb.Items))
	for i, item := range sb.Items {
		pResults := make([]corev1.ScoreboardProblemResultModel, len(item.ProblemResults))
		for j, r := range item.ProblemResults {
			penaltyOpt := corev1.NewOptInt32(int32(r.Penalty))

			var timeMinOpt corev1.OptInt32
			if r.TimeMinutes != nil {
				timeMinOpt = corev1.NewOptInt32(*r.TimeMinutes)
			}

			pResults[j] = corev1.ScoreboardProblemResultModel{
				ProblemID:       r.ProblemID,
				Solved:          r.Solved,
				FailedAttempts:  r.FailedAttempts,
				PendingAttempts: r.PendingAttempts,
				FirstAcTime:     timePtrToOptDateTime(r.FirstACTime),
				TimeMinutes:     timeMinOpt,
				Penalty:         penaltyOpt,
			}
		}

		items[i] = corev1.ScoreboardItemModel{
			UserID:         item.UserID,
			Username:       item.Username,
			ProblemsSolved: item.ProblemsSolved,
			TotalPenalty:   item.TotalPenalty,
			LastAcceptedAt: timePtrToOptDateTime(item.LastAcceptedAt),
			ProblemResults: pResults,
		}
		if item.Virtual {
			items[i].Virtual = corev1.NewOptBool(true)
		}
	}

	return &corev1.ScoreboardResponseModel{
		ContestID:         sb.ContestID,
		ContestLogin:      sb.ContestLogin,
		OrganizationLogin: sb.OrganizationLogin,
		PenaltyPerAttempt: sb.PenaltyPerAttempt,
		IsFrozen:          sb.IsFrozen,
		FreezeTime:        timePtrToOptNilDateTime(sb.FreezeTime),
		Problems:          problems,
		Items:             items,
	}
}

func OrganizationJoinRequestDTO(req organizationsmodel.OrganizationJoinRequest) corev1.OrganizationJoinRequestModel {
	return corev1.OrganizationJoinRequestModel{
		ID:                req.ID,
		OrganizationID:    req.OrganizationID,
		OrganizationName:  req.OrganizationName,
		OrganizationLogin: req.OrganizationLogin,
		UserID:            req.UserID,
		Username:          req.Username,
		Email:             stringToOptString(req.Email),
		Message:           stringPtrToOptString(req.Message),
		Status:            string(req.Status),
		ReviewerUsername:  stringPtrToOptString(req.ReviewerUsername),
		CreatedAt:         req.CreatedAt,
		UpdatedAt:         req.UpdatedAt,
	}
}

func ListOrganizationJoinRequestsResponseDTO(reqs []organizationsmodel.OrganizationJoinRequest) *corev1.ListOrganizationJoinRequestsResponseModel {
	items := make([]corev1.OrganizationJoinRequestModel, len(reqs))
	for i, r := range reqs {
		items[i] = OrganizationJoinRequestDTO(r)
	}
	return &corev1.ListOrganizationJoinRequestsResponseModel{
		Requests: items,
	}
}

func ContestJoinRequestDTO(req contestsmodel.ContestJoinRequest) corev1.ContestJoinRequestModel {
	return corev1.ContestJoinRequestModel{
		ID:                req.ID,
		ContestID:         req.ContestID,
		ContestTitle:      req.ContestTitle,
		ContestLogin:      req.ContestLogin,
		OrganizationLogin: req.OrganizationLogin,
		UserID:            req.UserID,
		Username:          req.Username,
		Email:             stringToOptString(req.Email),
		Message:           stringPtrToOptString(req.Message),
		Status:            string(req.Status),
		ReviewerUsername:  stringPtrToOptString(req.ReviewerUsername),
		CreatedAt:         req.CreatedAt,
		UpdatedAt:         req.UpdatedAt,
	}
}

func ListContestJoinRequestsResponseDTO(reqs []contestsmodel.ContestJoinRequest) *corev1.ListContestJoinRequestsResponseModel {
	items := make([]corev1.ContestJoinRequestModel, len(reqs))
	for i, r := range reqs {
		items[i] = ContestJoinRequestDTO(r)
	}
	return &corev1.ListContestJoinRequestsResponseModel{
		Requests: items,
	}
}

func ContestAnnouncementDTO(a *contestsmodel.ContestAnnouncement) *corev1.ContestAnnouncementModel {
	if a == nil {
		return nil
	}
	return &corev1.ContestAnnouncementModel{
		ID:             a.ID,
		ContestID:      a.ContestID,
		ProblemID:      uuidPtrToOptUUID(a.ProblemID),
		ProblemTitle:   stringPtrToOptString(a.ProblemTitle),
		ProblemLetter:  stringPtrToOptString(a.ProblemLetter),
		AuthorID:       a.AuthorID,
		AuthorUsername: a.AuthorUsername,
		Title:          a.Title,
		Body:           a.Body,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}

func ContestClarificationDTO(c *contestsmodel.ContestClarification) *corev1.ContestClarificationModel {
	if c == nil {
		return nil
	}
	return &corev1.ContestClarificationModel{
		ID:                 c.ID,
		ContestID:          c.ContestID,
		ProblemID:          uuidPtrToOptUUID(c.ProblemID),
		ProblemTitle:       stringPtrToOptString(c.ProblemTitle),
		ProblemLetter:      stringPtrToOptString(c.ProblemLetter),
		UserID:             c.UserID,
		Username:           c.Username,
		Question:           c.Question,
		Answer:             stringPtrToOptString(c.Answer),
		AnsweredBy:         uuidPtrToOptUUID(c.AnsweredBy),
		AnsweredByUsername: stringPtrToOptString(c.AnsweredByUsername),
		Status:             string(c.Status),
		CreatedAt:          c.CreatedAt,
		AnsweredAt:         timePtrToOptDateTime(c.AnsweredAt),
		UpdatedAt:          c.UpdatedAt,
	}
}

func usersListDTO(ul *identitymodel.UsersList) corev1.ListUsersResponseModel {
	userDTOs := make([]corev1.UserModel, len(ul.Users))
	for i, user := range ul.Users {
		userDTOs[i] = userProfileDTO(user)
	}

	return corev1.ListUsersResponseModel{
		Users:      userDTOs,
		Pagination: PaginationDTO(ul.Pagination),
	}
}

func contestTeamDTO(ct contestsmodel.ContestTeam) corev1.ContestTeamModel {
	return corev1.ContestTeamModel{
		ContestID:   ct.ContestID,
		TeamID:      ct.TeamID,
		TeamName:    ct.TeamName,
		TeamSlug:    ct.TeamSlug,
		ContestRole: corev1.ContestTeamModelContestRole(ct.Role),

		CreatedAt: ct.CreatedAt,
	}
}

func listContestTeamsDTO(teams []contestsmodel.ContestTeam) *corev1.ListContestTeamsResponseModel {
	resp := corev1.ListContestTeamsResponseModel{
		Teams: make([]corev1.ContestTeamModel, len(teams)),
	}
	for i, team := range teams {
		resp.Teams[i] = contestTeamDTO(team)
	}
	return &resp
}

func problemMemberDTO(pm problemsmodel.ProblemMember) corev1.ProblemMemberModel {
	return corev1.ProblemMemberModel{
		ProblemID: pm.ProblemID,
		UserID:    pm.UserID,
		Username:  pm.Username,
		Role:      corev1.ProblemMemberModelRole(pm.Role),
		CreatedAt: pm.CreatedAt,
	}
}

func listProblemMembersDTO(members []problemsmodel.ProblemMember, pagination paging.Pagination) *corev1.ListProblemMembersResponseModel {
	resp := corev1.ListProblemMembersResponseModel{
		Members:    make([]corev1.ProblemMemberModel, len(members)),
		Pagination: PaginationDTO(pagination),
	}
	for i, member := range members {
		resp.Members[i] = problemMemberDTO(member)
	}
	return &resp
}

func ContestDraftDTO(draft contestsmodel.ContestDraft) corev1.ContestDraftModel {
	return corev1.ContestDraftModel{
		ID:        draft.ID,
		UserID:    draft.UserID,
		Username:  stringToOptString(draft.Username),
		ContestID: draft.ContestID,
		Code:      draft.Code,
		CreatedAt: draft.CreatedAt,
		UpdatedAt: draft.UpdatedAt,
	}
}

func ListContestDraftsResponseDTO(list *contestsmodel.ContestDraftsList) *corev1.ListContestDraftsResponseModel {
	if list == nil {
		return &corev1.ListContestDraftsResponseModel{
			Drafts: []corev1.ContestDraftModel{},
			Pagination: corev1.PaginationModel{
				Page:       1,
				TotalItems: 0,
				TotalPages: 0,
				PageSize:   50,
			},
		}
	}

	drafts := make([]corev1.ContestDraftModel, len(list.Drafts))
	for i, d := range list.Drafts {
		drafts[i] = ContestDraftDTO(d)
	}

	return &corev1.ListContestDraftsResponseModel{
		Drafts:     drafts,
		Pagination: PaginationDTO(list.Pagination),
	}
}

func NotificationDTO(n notificationsmodel.Notification) corev1.NotificationModel {
	var optData corev1.OptNotificationModelData
	if len(n.Data) > 0 {
		dataMap := make(corev1.NotificationModelData)
		for k, v := range n.Data {
			if b, err := json.Marshal(v); err == nil {
				dataMap[k] = jx.Raw(b)
			}
		}
		optData = corev1.NewOptNotificationModelData(dataMap)
	}

	return corev1.NotificationModel{
		ID:        n.ID,
		UserID:    n.UserID,
		Type:      string(n.Type),
		Title:     n.Title,
		Body:      n.Body,
		Link:      stringPtrToOptString(n.Link),
		Data:      optData,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}
}

func NotificationsListResponseDTO(list *notificationsmodel.NotificationsList) *corev1.NotificationsListResponseModel {
	if list == nil {
		return &corev1.NotificationsListResponseModel{
			Notifications: []corev1.NotificationModel{},
			Pagination: corev1.PaginationModel{
				Page:       1,
				TotalItems: 0,
				TotalPages: 0,
				PageSize:   50,
			},
		}
	}

	notifs := make([]corev1.NotificationModel, len(list.Notifications))
	for i, n := range list.Notifications {
		notifs[i] = NotificationDTO(n)
	}

	return &corev1.NotificationsListResponseModel{
		Notifications: notifs,
		Pagination:    PaginationDTO(list.Pagination),
	}
}

func ContestAnnouncementsListResponseDTO(list *contestsmodel.ContestAnnouncementsList) *corev1.ListContestAnnouncementsResponseModel {
	if list == nil {
		return &corev1.ListContestAnnouncementsResponseModel{
			Announcements: []corev1.ContestAnnouncementModel{},
			Pagination:    PaginationDTO(paging.NewPagination(1, 50, 0)),
		}
	}

	items := make([]corev1.ContestAnnouncementModel, len(list.Announcements))
	for i, a := range list.Announcements {
		items[i] = *ContestAnnouncementDTO(&a)
	}

	return &corev1.ListContestAnnouncementsResponseModel{
		Announcements: items,
		Pagination:    PaginationDTO(list.Pagination),
	}
}

func ContestClarificationsListResponseDTO(list *contestsmodel.ContestClarificationsList) *corev1.ListContestClarificationsResponseModel {
	if list == nil {
		return &corev1.ListContestClarificationsResponseModel{
			Clarifications: []corev1.ContestClarificationModel{},
			Pagination:     PaginationDTO(paging.NewPagination(1, 50, 0)),
		}
	}

	items := make([]corev1.ContestClarificationModel, len(list.Clarifications))
	for i, c := range list.Clarifications {
		items[i] = *ContestClarificationDTO(&c)
	}

	return &corev1.ListContestClarificationsResponseModel{
		Clarifications: items,
		Pagination:     PaginationDTO(list.Pagination),
	}
}
