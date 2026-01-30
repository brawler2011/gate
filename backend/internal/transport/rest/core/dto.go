package core

import (
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

func GetContestResponseDTO(contest models.Contest, problems []models.ContestProblem, owner *models.User) *corev1.GetContestResponseModel {
	resp := corev1.GetContestResponseModel{
		Contest:  ContestDTO(contest, owner),
		Problems: make([]corev1.ContestProblemListItemModel, len(problems)),
	}

	for i, task := range problems {
		resp.Problems[i] = ContestProblemsListItemDTO(task)
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

func GetContestProblemResponseDTO(p models.ContestProblem) *corev1.GetContestProblemResponseModel {
	// Extract english title from titles map
	title := ""
	if t, ok := p.Titles["en"]; ok {
		title = t
	}

	position := int32(p.Ordinal)

	return &corev1.GetContestProblemResponseModel{
		Problem: corev1.ContestProblemModel{
			ProblemId:        p.ProblemID,
			Title:            title,
			TimeLimit:        0, // Not available in new model
			MemoryLimit:      0, // Not available in new model
			Position:         position,
			LegendHtml:       "", // Not available in new model
			InputFormatHtml:  "", // Not available in new model
			OutputFormatHtml: "", // Not available in new model
			NotesHtml:        "", // Not available in new model
			ScoringHtml:      "", // Not available in new model
			CreatedAt:        p.CreatedAt,
			UpdatedAt:        p.CreatedAt, // UpdatedAt not available in new model
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
	// Extract title from titles map
	title := ""
	if t, ok := c.Titles["en"]; ok {
		title = t
	}

	// Extract owner ID
	var createdBy uuid.UUID
	if c.OwnerID != nil {
		createdBy = *c.OwnerID
	}

	// Convert visibility
	visibility := string(c.Visibility)

	model := corev1.ContestModel{
		Id:                     c.ID,
		Title:                  title,
		Description:            c.Description,
		Visibility:             visibility,
		MonitorScope:           string(c.MonitorScope()),
		SubmissionsListScope:   string(c.SubmissionsListScope()),
		SubmissionsReviewScope: string(c.SubmissionsReviewScope()),
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

func ContestProblemsListItemDTO(t models.ContestProblem) corev1.ContestProblemListItemModel {
	// Extract title from titles map
	title := ""
	if tt, ok := t.Titles["en"]; ok {
		title = tt
	}

	return corev1.ContestProblemListItemModel{
		ProblemId:   t.ProblemID,
		Position:    int32(t.Ordinal),
		Title:       title,
		MemoryLimit: 0, // Not available
		TimeLimit:   0, // Not available
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.CreatedAt, // Not available
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
	// Extract title from titles map
	title := ""
	if t, ok := p.Titles["en"]; ok {
		title = t
	}

	return corev1.ProblemsListItemModel{
		Id:          p.ID,
		Title:       title,
		MemoryLimit: 0, // Not available in new model
		TimeLimit:   0, // Not available in new model
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func ProblemDTO(p models.Problem) *corev1.ProblemModel {
	// Extract title from titles map
	title := ""
	if t, ok := p.Titles["en"]; ok {
		title = t
	}

	return &corev1.ProblemModel{
		Id:          p.ID,
		Title:       title,
		TimeLimit:   0, // Not available in new model
		MemoryLimit: 0, // Not available in new model

		Legend:       "", // Not available in new model
		InputFormat:  "", // Not available in new model
		OutputFormat: "", // Not available in new model
		Notes:        "", // Not available in new model
		Scoring:      "", // Not available in new model

		LegendHtml:       "", // Not available in new model
		InputFormatHtml:  "", // Not available in new model
		OutputFormatHtml: "", // Not available in new model
		NotesHtml:        "", // Not available in new model
		ScoringHtml:      "", // Not available in new model

		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
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
