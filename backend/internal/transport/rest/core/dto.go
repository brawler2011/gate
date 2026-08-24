package core

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
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

func safeInt64[T ~uint64](v T) int64 {
	if uint64(v) > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
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

func PaginationDTO(p models.Pagination) corev1.PaginationModel {
	return corev1.PaginationModel{
		Page:  p.Page,
		Total: p.Total,
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

func GetContestResponseDTO(contest models.Contest, problems []models.ContestProblem, problemDetails map[uuid.UUID]models.Problem, owner *models.User) *corev1.GetContestResponseModel {
	resp := corev1.GetContestResponseModel{
		Contest:  ContestDTO(contest, owner),
		Problems: make([]corev1.ContestProblemListItemModel, len(problems)),
	}

	for i, task := range problems {
		if details, ok := problemDetails[task.ProblemID]; ok {
			resp.Problems[i] = ContestProblemsListItemDTO(task, &details)
			continue
		}

		resp.Problems[i] = ContestProblemsListItemDTO(task, nil)
	}

	return &resp
}

func ListContestsResponseDTO(contestsList *models.ContestsList) *corev1.ListContestsResponseModel {
	resp := corev1.ListContestsResponseModel{
		Contests:   make([]corev1.ContestModel, len(contestsList.Contests)),
		Pagination: PaginationDTO(contestsList.Pagination),
	}

	for i, contest := range contestsList.Contests {
		resp.Contests[i] = ContestDTO(contest, nil)
	}

	return &resp
}

func ListUserContestsResponseDTO(contestsList *models.ContestsList) *corev1.ListUserContestsResponseModel {
	resp := corev1.ListUserContestsResponseModel{
		Contests:   make([]corev1.ContestModel, len(contestsList.Contests)),
		Pagination: PaginationDTO(contestsList.Pagination),
	}

	for i, contest := range contestsList.Contests {
		resp.Contests[i] = ContestDTO(contest, nil)
	}

	return &resp
}

func GetContestProblemResponseDTO(contestProblem models.ContestProblem, problem models.Problem, statement *models.Statement, samples []corev1.ProblemSampleModel) *corev1.GetContestProblemResponseModel {
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
			ProblemID:        contestProblem.ProblemID,
			Title:            title,
			TimeLimit:        safeInt32(problem.TimeLimitMs),
			MemoryLimit:      safeInt32(problem.MemoryLimitMb),
			Position:         safeInt32(contestProblem.Ordinal),
			LegendHTML:       statementField(statement, func(s models.Statement) string { return s.Legend }),
			InputFormatHTML:  statementField(statement, func(s models.Statement) string { return s.InputFormat }),
			OutputFormatHTML: statementField(statement, func(s models.Statement) string { return s.OutputFormat }),
			NotesHTML:        statementField(statement, func(s models.Statement) string { return s.Notes }),
			ScoringHTML:      statementField(statement, func(s models.Statement) string { return s.Scoring }),
			CreatedAt:        problem.CreatedAt,
			UpdatedAt:        problem.UpdatedAt,
			Samples:          samples,
		},
	}
}

func SubmissionsListToDTO(submissionsList *models.SubmissionsList) *corev1.ListSubmissionsResponseModel {
	resp := corev1.ListSubmissionsResponseModel{
		Submissions: make([]corev1.SubmissionsListItemModel, len(submissionsList.Submissions)),
		Pagination:  PaginationDTO(submissionsList.Pagination),
	}

	for i, solution := range submissionsList.Submissions {
		resp.Submissions[i] = SubmissionListItemDTO(solution)
	}

	return &resp
}

func ContestDTO(c models.Contest, owner *models.User) corev1.ContestModel {
	title := c.Title

	// Extract owner ID
	var createdBy uuid.UUID
	if c.OwnerID != nil {
		createdBy = *c.OwnerID
	}

	// Convert visibility
	visibility := string(c.Visibility)
	settings := c.TypedSettings()
	monitorScope := "moderator"
	if settings.MonitorScope != "" {
		monitorScope = settings.MonitorScope
	}
	submissionsListScope := "moderator"
	if settings.SubmissionsListScope != "" {
		submissionsListScope = settings.SubmissionsListScope
	}
	submissionsReviewScope := "moderator"
	if settings.SubmissionsReviewScope != "" {
		submissionsReviewScope = settings.SubmissionsReviewScope
	}
	submissionDetailsScope := "moderator"
	if settings.SubmissionDetailsScope != "" {
		submissionDetailsScope = settings.SubmissionDetailsScope
	}

	freezeDurationMinutes := int32PtrToOptNilInt32(c.GetFreezeDurationMinutes())
	freezeStatus := corev1.ContestModelFreezeStatus(c.GetFreezeStatus())

	enableDrafts := corev1.NewOptBool(settings.GetEnableDrafts())
	allowClarifications := corev1.NewOptBool(settings.GetAllowClarifications())
	enableUpsolving := corev1.NewOptBool(settings.GetEnableUpsolving())
	enableVirtualContests := corev1.NewOptBool(settings.GetEnableVirtualContests())
	participationMode := corev1.NewOptContestModelParticipationMode(corev1.ContestModelParticipationMode(settings.GetParticipationMode()))
	hideStatements := corev1.NewOptBool(settings.GetHideStatements())

	model := corev1.ContestModel{
		ID:                     c.ID,
		Login:                  c.Login,
		OrganizationID:         c.OrganizationID,
		OrganizationLogin:      c.OrganizationLogin,
		Title:                  title,
		Description:            c.Description,
		Visibility:             visibility,
		MonitorScope:           monitorScope,
		SubmissionsListScope:   submissionsListScope,
		SubmissionsReviewScope: submissionsReviewScope,
		SubmissionDetailsScope: submissionDetailsScope,
		FreezeDurationMinutes:  freezeDurationMinutes,
		FreezeStatus:           freezeStatus,
		EnableDrafts:          enableDrafts,
		AllowClarifications:   allowClarifications,
		EnableUpsolving:        enableUpsolving,
		EnableVirtualContests: enableVirtualContests,
		ParticipationMode:     participationMode,
		HideStatements:        hideStatements,
		CreatedBy:              createdBy,
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
		StartTime:              timePtrToOptNilDateTime(c.StartTime),
		EndTime:                timePtrToOptNilDateTime(c.EndTime),
	}

	if owner != nil {
		ownerModel := UserDTO(*owner)
		model.Owner = corev1.NewOptUserModel(ownerModel)
	}

	return model
}

func ContestProblemsListItemDTO(t models.ContestProblem, problem *models.Problem) corev1.ContestProblemListItemModel {
	title := strings.TrimSpace(t.Title)
	if problem != nil {
		problemTitle := strings.TrimSpace(problem.Title)
		if problemTitle != "" {
			title = problemTitle
		}
	}

	timeLimit := int32(0)
	memoryLimit := int32(0)
	updatedAt := t.CreatedAt
	if problem != nil {
		timeLimit = safeInt32(problem.TimeLimitMs)
		memoryLimit = safeInt32(problem.MemoryLimitMb)
		updatedAt = problem.UpdatedAt
	}

	return corev1.ContestProblemListItemModel{
		ProblemID:   t.ProblemID,
		Position:    safeInt32(t.Ordinal),
		Title:       title,
		MemoryLimit: memoryLimit,
		TimeLimit:   timeLimit,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   updatedAt,
	}
}

func UserDTO(u models.User) corev1.UserModel {
	return userDTO(u)
}

func ParticipantDTO(p models.ContestMember) corev1.UserModel {
	return corev1.UserModel{
		ID:        p.UserID,
		Username:  p.Username,
		Role:      string(p.Role),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func ProblemsListItemDTO(p models.Problem) corev1.ProblemsListItemModel {
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

func ListProblemsResponseDTO(problemsList *models.ProblemsList) *corev1.ListProblemsResponseModel {
	resp := corev1.ListProblemsResponseModel{
		Problems:   make([]corev1.ProblemsListItemModel, len(problemsList.Problems)),
		Pagination: PaginationDTO(problemsList.Pagination),
	}

	for i, problem := range problemsList.Problems {
		resp.Problems[i] = ProblemsListItemDTO(problem)
	}

	return &resp
}

func ProblemDTO(p models.Problem, statement *models.Statement, samples []corev1.ProblemSampleModel) *corev1.ProblemModel {
	title := strings.TrimSpace(p.Title)
	if statement != nil {
		statementTitle := strings.TrimSpace(statement.Title)
		if statementTitle != "" {
			title = statementTitle
		}
	}

	createdBy := uuid.Nil
	if p.OwnerID != nil {
		createdBy = *p.OwnerID
	}

	legend := statementField(statement, func(s models.Statement) string { return s.Legend })
	inputFormat := statementField(statement, func(s models.Statement) string { return s.InputFormat })
	outputFormat := statementField(statement, func(s models.Statement) string { return s.OutputFormat })
	notes := statementField(statement, func(s models.Statement) string { return s.Notes })
	scoring := statementField(statement, func(s models.Statement) string { return s.Scoring })

	if samples == nil {
		samples = []corev1.ProblemSampleModel{}
	}

	return &corev1.ProblemModel{
		ID:                p.ID,
		OrganizationID:    p.OrganizationID,
		OrganizationLogin: p.OrganizationLogin,
		Title:             title,
		Visibility:        p.Visibility,
		CreatedBy:         createdBy,
		TimeLimit:         safeInt32(p.TimeLimitMs),
		MemoryLimit:       safeInt32(p.MemoryLimitMb),

		Legend:       legend,
		InputFormat:  inputFormat,
		OutputFormat: outputFormat,
		Notes:        notes,
		Scoring:      scoring,

		LegendHTML:       legend,
		InputFormatHTML:  inputFormat,
		OutputFormatHTML: outputFormat,
		NotesHTML:        notes,
		ScoringHTML:      scoring,

		IsTemplate: p.IsTemplate,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
		Samples:    samples,
	}
}

func statementField(statement *models.Statement, getter func(models.Statement) string) string {
	if statement == nil {
		return ""
	}

	return getter(*statement)
}

func SubmissionListItemDTO(s models.Submission) corev1.SubmissionsListItemModel {
	return corev1.SubmissionsListItemModel{
		ID:     s.ID,
		UserID: uuidPtrToUUID(s.CreatedBy),

		Username: s.Username,

		State:      s.State,
		Score:      s.Score,
		Penalty:    s.Penalty,
		TimeStat:   s.TimeStat,
		MemoryStat: s.MemoryStat,
		Language:   s.Language,
		FailedTest: int32PtrToOptNilInt32(s.FailedTest),

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
}

func SubmissionTestDetailsDTO(td *models.SubmissionTestDetails) *corev1.SubmissionTestDetailsModel {
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
			}
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
		ErrorLine:         int32PtrToOptNilInt32(td.ErrorLine),
		Tests:             tests,
		FailedTestDetails: failedTestDetails,
	}
}

func SolutionDTO(s models.Submission) corev1.SubmissionModel {
	var testDetailsOpt corev1.OptSubmissionTestDetailsModel
	if td := SubmissionTestDetailsDTO(s.TestDetails); td != nil {
		testDetailsOpt = corev1.NewOptSubmissionTestDetailsModel(*td)
	}

	return corev1.SubmissionModel{
		ID:     s.ID,
		UserID: uuidPtrToUUID(s.CreatedBy),

		Username: s.Username,

		Submission: s.Submission,

		State:       s.State,
		Score:       s.Score,
		Penalty:     s.Penalty,
		TimeStat:    s.TimeStat,
		MemoryStat:  s.MemoryStat,
		Language:    s.Language,
		FailedTest:  int32PtrToOptNilInt32(s.FailedTest),
		TestDetails: testDetailsOpt,

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
}

func userDTO(u models.User) corev1.UserModel {
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
		ImgId:           imgID,
		ExpiresAt:       timePtrToOptDateTime(u.ExpiresAt),
		ClaimedByUserID: uuidPtrToOptUUID(u.ClaimedByUserID),
		ClaimedAt:       timePtrToOptDateTime(u.ClaimedAt),
		IsEmailVerified: corev1.NewOptBool(u.IsEmailVerified),
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}

func usersListDTO(ul *models.UsersList) corev1.ListUsersResponseModel {
	userDTOs := make([]corev1.UserModel, len(ul.Users))
	for i, user := range ul.Users {
		userDTOs[i] = userDTO(user)
	}

	return corev1.ListUsersResponseModel{
		Users: userDTOs,
		Pagination: corev1.PaginationModel{
			Page:  ul.Pagination.Page,
			Total: ul.Pagination.Total,
		},
	}
}

// Organizations DTOs

func organizationDTO(o models.Organization) corev1.OrganizationModel {
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

func listOrganizationsDTO(ol *models.OrganizationList) *corev1.ListOrganizationsResponseModel {
	resp := corev1.ListOrganizationsResponseModel{
		Organizations: make([]corev1.OrganizationModel, len(ol.Organizations)),
		Pagination:    PaginationDTO(ol.Pagination),
	}

	for i, org := range ol.Organizations {
		resp.Organizations[i] = organizationDTO(org)
	}

	return &resp
}

func organizationMemberDTO(m models.OrganizationMember) corev1.OrganizationMemberModel {
	return corev1.OrganizationMemberModel{
		UserID:         m.UserID,
		OrganizationID: m.OrganizationID,
		Username:       m.Username,
		Role:           string(m.Role),
		CreatedAt:      m.CreatedAt,
	}
}

func listOrganizationMembersDTO(members []models.OrganizationMember, page, total int32) *corev1.ListOrganizationMembersResponseModel {
	resp := corev1.ListOrganizationMembersResponseModel{
		Members: make([]corev1.OrganizationMemberModel, len(members)),
		Pagination: corev1.PaginationModel{
			Page:  page,
			Total: total,
		},
	}

	for i, member := range members {
		resp.Members[i] = organizationMemberDTO(member)
	}

	return &resp
}

// Teams DTOs

func teamDTO(t models.Team) corev1.TeamModel {
	return corev1.TeamModel{
		ID:             t.ID,
		Name:           t.Name,
		OrganizationID: t.OrganizationID,
		Description:    stringToOptString(t.Description),
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

func listTeamsDTO(teams []models.Team, page, total int32) *corev1.ListTeamsResponseModel {
	resp := corev1.ListTeamsResponseModel{
		Teams: make([]corev1.TeamModel, len(teams)),
		Pagination: corev1.PaginationModel{
			Page:  page,
			Total: total,
		},
	}

	for i, team := range teams {
		resp.Teams[i] = teamDTO(team)
	}

	return &resp
}

func teamMemberDTO(m models.TeamMember) corev1.TeamMemberModel {
	return corev1.TeamMemberModel{
		UserID:    m.UserID,
		TeamID:    m.TeamID,
		Username:  m.Username,
		Role:      string(m.Role),
		CreatedAt: m.CreatedAt,
	}
}

func listTeamMembersDTO(members []models.TeamMember, page, total int32) *corev1.ListTeamMembersResponseModel {
	resp := corev1.ListTeamMembersResponseModel{
		Members: make([]corev1.TeamMemberModel, len(members)),
		Pagination: corev1.PaginationModel{
			Page:  page,
			Total: total,
		},
	}

	for i, member := range members {
		resp.Members[i] = teamMemberDTO(member)
	}

	return &resp
}

func contestTeamDTO(ct models.ContestTeam) corev1.ContestTeamModel {
	var mask corev1.OptInt64
	if ct.PermissionsMask != nil {
		mask = corev1.NewOptInt64(int64(*ct.PermissionsMask)) //nolint:gosec // bitmask conversion
	}
	return corev1.ContestTeamModel{
		ContestID:       ct.ContestID,
		TeamID:          ct.TeamID,
		TeamName:        ct.TeamName,
		TeamSlug:        ct.TeamSlug,
		ContestRole:     string(ct.Role),
		PermissionsMask: mask,
		CreatedAt:       ct.CreatedAt,
	}
}

func listContestTeamsDTO(teams []models.ContestTeam) *corev1.ListContestTeamsResponseModel {
	resp := corev1.ListContestTeamsResponseModel{
		Teams: make([]corev1.ContestTeamModel, len(teams)),
	}
	for i, team := range teams {
		resp.Teams[i] = contestTeamDTO(team)
	}
	return &resp
}

func problemTeamDTO(pt models.ProblemTeam) corev1.ProblemTeamModel {
	return corev1.ProblemTeamModel{
		ProblemID:  pt.ProblemID,
		TeamID:     pt.TeamID,
		TeamName:   pt.TeamName,
		TeamSlug:   pt.TeamSlug,
		Permission: string(pt.Permission),
		CreatedAt:  pt.CreatedAt,
	}
}

func listProblemTeamsDTO(teams []models.ProblemTeam) *corev1.ListProblemTeamsResponseModel {
	resp := corev1.ListProblemTeamsResponseModel{
		Teams: make([]corev1.ProblemTeamModel, len(teams)),
	}
	for i, team := range teams {
		resp.Teams[i] = problemTeamDTO(team)
	}
	return &resp
}

func problemMemberDTO(pm models.ProblemMember) corev1.ProblemMemberModel {
	return corev1.ProblemMemberModel{
		ProblemID: pm.ProblemID,
		UserID:    pm.UserID,
		Username:  pm.Username,
		Role:      string(pm.Role),
		CreatedAt: pm.CreatedAt,
	}
}

func listProblemMembersDTO(members []models.ProblemMember, page, total int32) *corev1.ListProblemMembersResponseModel {
	resp := corev1.ListProblemMembersResponseModel{
		Members: make([]corev1.ProblemMemberModel, len(members)),
		Pagination: corev1.PaginationModel{
			Page:  page,
			Total: total,
		},
	}
	for i, member := range members {
		resp.Members[i] = problemMemberDTO(member)
	}
	return &resp
}

func DashboardContestDTO(c models.DashboardContest) corev1.DashboardContestModel {
	return corev1.DashboardContestModel{
		ID:                 c.ID,
		Login:              c.Login,
		Title:              c.Title,
		OrganizationID:     c.OrganizationID,
		OrganizationName:   c.OrganizationName,
		OrganizationLogin:  c.OrganizationLogin,
		UserRole:           c.UserRole,
		StartTime:          timePtrToOptNilDateTime(c.StartTime),
		EndTime:            timePtrToOptNilDateTime(c.EndTime),
		LastSubmissionTime: timePtrToOptNilDateTime(c.LastSubmissionTime),
		CreatedAt:          c.CreatedAt,
	}
}

func DashboardProblemDTO(p models.DashboardProblem) corev1.DashboardProblemModel {
	return corev1.DashboardProblemModel{
		ID:                p.ID,
		Title:             p.Title,
		OrganizationID:    p.OrganizationID,
		OrganizationName:  p.OrganizationName,
		OrganizationLogin: p.OrganizationLogin,
		TimeLimit:         safeInt32(p.TimeLimitMs),
		MemoryLimit:       safeInt32(p.MemoryLimitMb),
		UpdatedAt:         p.UpdatedAt,
	}
}

func DashboardResponseDTO(contests []models.DashboardContest, problems []models.DashboardProblem) corev1.GetUserDashboardResponseModel {
	contestDTOs := make([]corev1.DashboardContestModel, len(contests))
	for i, c := range contests {
		contestDTOs[i] = DashboardContestDTO(c)
	}

	problemDTOs := make([]corev1.DashboardProblemModel, len(problems))
	for i, p := range problems {
		problemDTOs[i] = DashboardProblemDTO(p)
	}

	return corev1.GetUserDashboardResponseModel{
		RecentContests: contestDTOs,
		MyProblems:     problemDTOs,
	}
}

func GetScoreboardResponseDTO(sb *models.ScoreboardResponse) *corev1.ScoreboardResponseModel {
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

func ContestDraftDTO(draft models.ContestDraft) corev1.ContestDraftModel {
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

func ListContestDraftsResponseDTO(list *models.ContestDraftsList) *corev1.ListContestDraftsResponseModel {
	if list == nil {
		return &corev1.ListContestDraftsResponseModel{
			Drafts: []corev1.ContestDraftModel{},
			Pagination: corev1.PaginationModel{
				Page:  1,
				Total: 0,
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

func NotificationDTO(n models.Notification) corev1.NotificationModel {
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

func NotificationsListResponseDTO(list *models.NotificationsList) *corev1.NotificationsListResponseModel {
	if list == nil {
		return &corev1.NotificationsListResponseModel{
			Notifications: []corev1.NotificationModel{},
			Pagination: corev1.PaginationModel{
				Page:  1,
				Total: 0,
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

func OrganizationInvitationDTO(inv models.OrganizationInvitation) corev1.OrganizationInvitationModel {
	return corev1.OrganizationInvitationModel{
		ID:                    inv.ID,
		OrganizationID:        inv.OrganizationID,
		OrganizationName:      inv.OrganizationName,
		OrganizationLogin:     inv.OrganizationLogin,
		OrganizationAvatarURL: stringPtrToOptString(inv.OrganizationAvatarURL),
		UserID:                inv.UserID,
		Username:              inv.Username,
		Email:                 stringToOptString(inv.Email),
		InviterID:             inv.InviterID,
		InviterUsername:       inv.InviterUsername,
		Role:                  string(inv.Role),
		Status:                string(inv.Status),
		CreatedAt:             inv.CreatedAt,
		UpdatedAt:             inv.UpdatedAt,
	}
}

func ListOrganizationInvitationsResponseDTO(invs []models.OrganizationInvitation) *corev1.ListOrganizationInvitationsResponseModel {
	items := make([]corev1.OrganizationInvitationModel, len(invs))
	for i, inv := range invs {
		items[i] = OrganizationInvitationDTO(inv)
	}
	return &corev1.ListOrganizationInvitationsResponseModel{
		Invitations: items,
	}
}

func OrganizationJoinRequestDTO(req models.OrganizationJoinRequest) corev1.OrganizationJoinRequestModel {
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

func ListOrganizationJoinRequestsResponseDTO(reqs []models.OrganizationJoinRequest) *corev1.ListOrganizationJoinRequestsResponseModel {
	items := make([]corev1.OrganizationJoinRequestModel, len(reqs))
	for i, r := range reqs {
		items[i] = OrganizationJoinRequestDTO(r)
	}
	return &corev1.ListOrganizationJoinRequestsResponseModel{
		Requests: items,
	}
}

func ContestJoinRequestDTO(req models.ContestJoinRequest) corev1.ContestJoinRequestModel {
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

func ListContestJoinRequestsResponseDTO(reqs []models.ContestJoinRequest) *corev1.ListContestJoinRequestsResponseModel {
	items := make([]corev1.ContestJoinRequestModel, len(reqs))
	for i, r := range reqs {
		items[i] = ContestJoinRequestDTO(r)
	}
	return &corev1.ListContestJoinRequestsResponseModel{
		Requests: items,
	}
}

func ContestAnnouncementDTO(a *models.ContestAnnouncement) *corev1.ContestAnnouncementModel {
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

func ContestAnnouncementsListResponseDTO(list *models.ContestAnnouncementsList) *corev1.ListContestAnnouncementsResponseModel {
	if list == nil {
		return &corev1.ListContestAnnouncementsResponseModel{
			Announcements: []corev1.ContestAnnouncementModel{},
			Pagination:    corev1.PaginationModel{},
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

func ContestClarificationDTO(c *models.ContestClarification) *corev1.ContestClarificationModel {
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

func ContestClarificationsListResponseDTO(list *models.ContestClarificationsList) *corev1.ListContestClarificationsResponseModel {
	if list == nil {
		return &corev1.ListContestClarificationsResponseModel{
			Clarifications: []corev1.ContestClarificationModel{},
			Pagination:     corev1.PaginationModel{},
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





