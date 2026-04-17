package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
)

func (h *CoreServer) GetUser(ctx context.Context, request corev1.GetUserRequestObject) (corev1.GetUserResponseObject, error) {
	user, err := h.usersUC.GetUserById(ctx, request.Id)
	if err != nil {
		return nil, err
	}

	return corev1.GetUser200JSONResponse{
		User: userDTO(user),
	}, nil
}

func (h *CoreServer) GetMe(ctx context.Context, request corev1.GetMeRequestObject) (corev1.GetMeResponseObject, error) {
	user := middleware.GetUser(ctx)

	return corev1.GetMe200JSONResponse{
		User: userDTO(user),
	}, nil
}

func (h *CoreServer) PatchMe(ctx context.Context, request corev1.PatchMeRequestObject) (corev1.PatchMeResponseObject, error) {
	user := middleware.GetUser(ctx)

	err := h.usersUC.UpdateUser(ctx, models.UpdateUserInput{
		Id:      user.Id,
		Name:    request.Body.Name,
		Surname: request.Body.Surname,
		Bio:     request.Body.Bio,
	})
	if err != nil {
		return nil, err
	}

	return corev1.PatchMe200Response{}, nil
}

func (h *CoreServer) ListUsers(ctx context.Context, request corev1.ListUsersRequestObject) (corev1.ListUsersResponseObject, error) {
	filter, err := validateGetUsersParams(request.Params)
	if err != nil {
		return nil, err
	}

	users, err := h.usersUC.ListUsers(ctx, *filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListUsers200JSONResponse(usersListDTO(&users)), nil
}

func (h *CoreServer) ListUserSubmissions(ctx context.Context, request corev1.ListUserSubmissionsRequestObject) (corev1.ListUserSubmissionsResponseObject, error) {
	filter := listUserSubmissionsParamsToFilter(request.UserId, request.Params)

	submissions, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListUserSubmissions200JSONResponse(*ListSolutionsResponseDTO(submissions)), nil
}
