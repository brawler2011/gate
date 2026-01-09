package core

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
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
	return &corev1.GetContestProblemResponseModel{
		Problem: corev1.ContestProblemModel{
			ProblemId:        p.ProblemID,
			Title:            p.Title,
			TimeLimit:        p.TimeLimit,
			MemoryLimit:      p.MemoryLimit,
			Position:         p.Position,
			LegendHtml:       p.LegendHtml,
			InputFormatHtml:  p.InputFormatHtml,
			OutputFormatHtml: p.OutputFormatHtml,
			NotesHtml:        p.NotesHtml,
			ScoringHtml:      p.ScoringHtml,
			CreatedAt:        p.CreatedAt,
			UpdatedAt:        p.UpdatedAt,
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
	model := corev1.ContestModel{
		Id:                     c.ID,
		Title:                  c.Title,
		Description:            c.Description,
		Visibility:             c.Visibility,
		MonitorScope:           c.MonitorScope,
		SubmissionsListScope:   c.SubmissionsListScope,
		SubmissionsReviewScope: c.SubmissionsReviewScope,
		CreatedBy:              c.CreatedBy,
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
	return corev1.ContestProblemListItemModel{
		ProblemId:   t.ProblemID,
		Position:    t.Position,
		Title:       t.Title,
		MemoryLimit: t.MemoryLimit,
		TimeLimit:   t.TimeLimit,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
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
	return corev1.ProblemsListItemModel{
		Id:          p.ID,
		Title:       p.Title,
		MemoryLimit: p.MemoryLimit,
		TimeLimit:   p.TimeLimit,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func ProblemDTO(p models.Problem) *corev1.ProblemModel {
	return &corev1.ProblemModel{
		Id:          p.ID,
		Title:       p.Title,
		TimeLimit:   p.TimeLimit,
		MemoryLimit: p.MemoryLimit,

		Legend:       p.Legend,
		InputFormat:  p.InputFormat,
		OutputFormat: p.OutputFormat,
		Notes:        p.Notes,
		Scoring:      p.Scoring,

		LegendHtml:       p.LegendHtml,
		InputFormatHtml:  p.InputFormatHtml,
		OutputFormatHtml: p.OutputFormatHtml,
		NotesHtml:        p.NotesHtml,
		ScoringHtml:      p.ScoringHtml,

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
		ImgId:     u.ImgId,
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
