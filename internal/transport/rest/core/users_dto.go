package handlers

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

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
