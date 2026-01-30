package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
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
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	return corev1.GetMe200JSONResponse{
		User: userDTO(user),
	}, nil
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
	user := middleware.GetUser(ctx)

	if request.UserId != user.Id {
		if !user.IsAdmin() {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "only admins can view other users' submissions")
		}
	} else if request.Params.ContestId != nil {
		canView, err := h.permissionsUC.HasContestPermission(ctx, *request.Params.ContestId, user.Id, models.ActionListOwnSubmissions)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
	}

	filter := listUserSubmissionsParamsToFilter(request.UserId, request.Params)

	submissions, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListUserSubmissions200JSONResponse(*ListSolutionsResponseDTO(submissions)), nil
}
