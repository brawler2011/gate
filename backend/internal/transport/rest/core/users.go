package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) GetUser(ctx context.Context, request corev1.GetUserRequestObject) (corev1.GetUserResponseObject, error) {
	user, err := h.usersUC.GetUserByUsername(ctx, request.Username)
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

func (h *CoreServer) GetMyDashboard(ctx context.Context, request corev1.GetMyDashboardRequestObject) (corev1.GetMyDashboardResponseObject, error) {
	user := middleware.GetUser(ctx)

	contests, err := h.contestsUC.ListDashboardContests(ctx, user.Id, 5)
	if err != nil {
		return nil, err
	}

	problems, err := h.problemsUC.ListDashboardProblems(ctx, user.Id, 5)
	if err != nil {
		return nil, err
	}

	return corev1.GetMyDashboard200JSONResponse(DashboardResponseDTO(contests, problems)), nil
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
	user, err := h.usersUC.GetUserByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}

	filter := listUserSubmissionsParamsToFilter(user.Id, request.Params)

	submissions, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListUserSubmissions200JSONResponse(*ListSolutionsResponseDTO(submissions)), nil
}

func (h *CoreServer) UpdateUser(ctx context.Context, request corev1.UpdateUserRequestObject) (corev1.UpdateUserResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing body")
	}

	user, err := h.usersUC.GetUserByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}

	input := models.UpdateUserInput{
		Id:       user.Id,
		Username: request.Body.Username,
		Email:    request.Body.Email,
	}
	if request.Body.Role != nil {
		r := string(*request.Body.Role)
		input.Role = &r
	}

	err = h.usersUC.UpdateUser(ctx, input)
	if err != nil {
		return nil, err
	}

	return corev1.UpdateUser200Response{}, nil
}

func (h *CoreServer) ClaimTemporaryUser(ctx context.Context, request corev1.ClaimTemporaryUserRequestObject) (corev1.ClaimTemporaryUserResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing body")
	}

	user := middleware.GetUser(ctx)

	result, err := h.usersUC.ClaimTemporaryUser(ctx, user, models.ClaimTemporaryUserInput{
		Username: request.Body.Username,
		Password: request.Body.Password,
	})
	if err != nil {
		return nil, err
	}

	var contestsGranted *[]uuid.UUID
	if len(result.ContestsGranted) > 0 {
		cg := make([]uuid.UUID, len(result.ContestsGranted))
		copy(cg, result.ContestsGranted)
		contestsGranted = &cg
	}

	return corev1.ClaimTemporaryUser200JSONResponse{
		ClaimedUserId:   result.ClaimedUserID,
		ClaimedUsername: result.ClaimedUsername,
		ContestsGranted: contestsGranted,
	}, nil
}

func (h *CoreServer) ListMyClaimedAccounts(ctx context.Context, request corev1.ListMyClaimedAccountsRequestObject) (corev1.ListMyClaimedAccountsResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	accounts, err := h.usersUC.ListClaimedAccounts(ctx, user.Id)
	if err != nil {
		return nil, err
	}

	items := make([]corev1.ClaimedAccountItem, len(accounts))
	for i, acc := range accounts {
		items[i] = corev1.ClaimedAccountItem{
			Id:        acc.ID,
			Username:  acc.Username,
			ClaimedAt: acc.ClaimedAt,
			ExpiresAt: acc.ExpiresAt,
		}
	}

	return corev1.ListMyClaimedAccounts200JSONResponse{
		Accounts: items,
	}, nil
}


