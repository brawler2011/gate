package core

import (
	"strings"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

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

func GetContestProblemResponseDTO(contestProblem models.ContestProblem, problem models.Problem, statement *models.Statement) *corev1.GetContestProblemResponseModel {
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

	return &corev1.GetContestProblemResponseModel{
		Problem: corev1.ContestProblemModel{
			ProblemId:        contestProblem.ProblemID,
			Title:            title,
			TimeLimit:        int32(problem.TimeLimitMs),
			MemoryLimit:      int32(problem.MemoryLimitMb),
			Position:         int32(contestProblem.Ordinal),
			LegendHtml:       statementField(statement, func(s models.Statement) string { return s.Legend }),
			InputFormatHtml:  statementField(statement, func(s models.Statement) string { return s.InputFormat }),
			OutputFormatHtml: statementField(statement, func(s models.Statement) string { return s.OutputFormat }),
			NotesHtml:        statementField(statement, func(s models.Statement) string { return s.Notes }),
			ScoringHtml:      statementField(statement, func(s models.Statement) string { return s.Scoring }),
			CreatedAt:        problem.CreatedAt,
			UpdatedAt:        problem.UpdatedAt,
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
	monitorScope := "moderator"
	submissionsListScope := "moderator"
	submissionsReviewScope := "moderator"

	if rawMonitorScope, ok := c.Settings["monitor_scope"]; ok {
		if parsedMonitorScope, ok := rawMonitorScope.(string); ok && parsedMonitorScope != "" {
			monitorScope = parsedMonitorScope
		}
	}

	if rawSubmissionsListScope, ok := c.Settings["submissions_list_scope"]; ok {
		if parsedSubmissionsListScope, ok := rawSubmissionsListScope.(string); ok && parsedSubmissionsListScope != "" {
			submissionsListScope = parsedSubmissionsListScope
		}
	}

	if rawSubmissionsReviewScope, ok := c.Settings["submissions_review_scope"]; ok {
		if parsedSubmissionsReviewScope, ok := rawSubmissionsReviewScope.(string); ok && parsedSubmissionsReviewScope != "" {
			submissionsReviewScope = parsedSubmissionsReviewScope
		}
	}

	model := corev1.ContestModel{
		Id:                     c.ID,
		OrganizationId:         &c.OrganizationID,
		Title:                  title,
		Description:            c.Description,
		Visibility:             visibility,
		MonitorScope:           monitorScope,
		SubmissionsListScope:   submissionsListScope,
		SubmissionsReviewScope: submissionsReviewScope,
		CreatedBy:              createdBy,
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
	}

	if owner != nil {
		ownerModel := UserDTO(*owner)
		model.Owner = &ownerModel
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
		timeLimit = int32(problem.TimeLimitMs)
		memoryLimit = int32(problem.MemoryLimitMb)
		updatedAt = problem.UpdatedAt
	}

	return corev1.ContestProblemListItemModel{
		ProblemId:   t.ProblemID,
		Position:    int32(t.Ordinal),
		Title:       title,
		MemoryLimit: memoryLimit,
		TimeLimit:   timeLimit,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   updatedAt,
	}
}

func UserDTO(u models.User) corev1.UserModel {
	return corev1.UserModel{
		Id:        u.Id,
		Username:  u.Username,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func ParticipantDTO(p models.ContestMember) corev1.UserModel {
	return corev1.UserModel{
		Id:        p.UserID,
		Username:  p.Username,
		Role:      string(p.Role),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func ProblemsListItemDTO(p models.Problem) corev1.ProblemsListItemModel {
	title := p.Title

	return corev1.ProblemsListItemModel{
		Id:          p.ID,
		Title:       title,
		Visibility:  &p.Visibility,
		MemoryLimit: int32(p.MemoryLimitMb),
		TimeLimit:   int32(p.TimeLimitMs),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func ProblemDTO(p models.Problem, statement *models.Statement) *corev1.ProblemModel {
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

	return &corev1.ProblemModel{
		Id:             p.ID,
		OrganizationId: &p.OrganizationID,
		Title:          title,
		Visibility:     p.Visibility,
		CreatedBy:      createdBy,
		TimeLimit:      int32(p.TimeLimitMs),
		MemoryLimit:    int32(p.MemoryLimitMb),

		Legend:       legend,
		InputFormat:  inputFormat,
		OutputFormat: outputFormat,
		Notes:        notes,
		Scoring:      scoring,

		LegendHtml:       legend,
		InputFormatHtml:  inputFormat,
		OutputFormatHtml: outputFormat,
		NotesHtml:        notes,
		ScoringHtml:      scoring,

		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
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
		Id: s.ID,

		Username: s.Username,

		State:      s.State,
		Score:      s.Score,
		Penalty:    s.Penalty,
		TimeStat:   s.TimeStat,
		MemoryStat: s.MemoryStat,
		Language:   s.Language,

		ProblemId:    uuidPtrToUUID(s.ProblemID),
		ProblemTitle: s.ProblemTitle,

		Position: int32PtrToInt32(s.Position),

		ContestId:    uuidPtrToUUID(s.ContestID),
		ContestTitle: s.ContestTitle,

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func SolutionDTO(s models.Submission) corev1.SubmissionModel {
	return corev1.SubmissionModel{
		Id: s.ID,

		Username: s.Username,

		Submission: s.Submission,

		State:      s.State,
		Score:      s.Score,
		Penalty:    s.Penalty,
		TimeStat:   s.TimeStat,
		MemoryStat: s.MemoryStat,
		Language:   s.Language,

		ProblemId:    uuidPtrToUUID(s.ProblemID),
		ProblemTitle: s.ProblemTitle,

		Position: int32PtrToInt32(s.Position),

		ContestId:    uuidPtrToUUID(s.ContestID),
		ContestTitle: s.ContestTitle,

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func userDTO(u models.User) corev1.UserModel {
	return corev1.UserModel{
		Id:        u.Id,
		Username:  u.Username,
		Role:      u.Role,
		Email:     &u.Email,
		Name:      &u.Name,
		Surname:   &u.Surname,
		Bio:       &u.Bio,
		ImgId:     nil, // Avatar URL not compatible with UUID type
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
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

func listUserSubmissionsParamsToFilter(userId uuid.UUID, params corev1.ListUserSubmissionsParams) models.SubmissionsFilter {
	var state *models.State = nil
	if params.State != nil {
		s := models.State(*params.State)
		state = &s
	}

	// Convert sortOrder string to integer: -1 for desc, 0 for asc
	var order *int32 = nil
	if params.SortOrder != nil {
		var orderVal int32
		if *params.SortOrder == corev1.ListUserSubmissionsParamsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	return models.SubmissionsFilter{
		ContestId: params.ContestId,
		Page:      params.Page,
		PageSize:  params.PageSize,
		ProblemId: params.ProblemId,
		UserId:    &userId,
		Language:  nil,
		Order:     order,
		State:     state,
	}
}

func submissionsListToDTO(solutionsList *models.SubmissionsList) *corev1.ListSubmissionsResponseModel {
	resp := corev1.ListSubmissionsResponseModel{
		Submissions: make([]corev1.SubmissionsListItemModel, len(solutionsList.Submissions)),
		Pagination: corev1.PaginationModel{
			Page:  solutionsList.Pagination.Page,
			Total: solutionsList.Pagination.Total,
		},
	}

	for i, solution := range solutionsList.Submissions {
		resp.Submissions[i] = corev1.SubmissionsListItemModel{
			Id:           solution.ID,
			UserId:       uuidPtrToUUID(solution.CreatedBy),
			Username:     solution.Username,
			State:        solution.State,
			Score:        solution.Score,
			Penalty:      solution.Penalty,
			TimeStat:     solution.TimeStat,
			MemoryStat:   solution.MemoryStat,
			Language:     solution.Language,
			ProblemId:    uuidPtrToUUID(solution.ProblemID),
			ProblemTitle: solution.ProblemTitle,
			Position:     int32PtrToInt32(solution.Position),
			ContestId:    uuidPtrToUUID(solution.ContestID),
			ContestTitle: solution.ContestTitle,
			UpdatedAt:    solution.UpdatedAt,
			CreatedAt:    solution.CreatedAt,
		}
	}

	return &resp
}

// Organizations DTOs

func organizationDTO(o models.Organization) corev1.OrganizationModel {
	description := ""
	if o.Description != "" {
		description = o.Description
	}

	return corev1.OrganizationModel{
		Id:          o.ID,
		Name:        o.Name,
		Description: &description,
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
		UserId:         m.UserID,
		OrganizationId: m.OrganizationID,
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
	description := ""
	if t.Description != "" {
		description = t.Description
	}

	return corev1.TeamModel{
		Id:             t.ID,
		Name:           t.Name,
		OrganizationId: t.OrganizationID,
		Description:    &description,
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
		UserId:    m.UserID,
		TeamId:    m.TeamID,
		Username:  m.Username,
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
