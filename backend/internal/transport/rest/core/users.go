package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) GetUser(ctx context.Context, params corev1.GetUserParams) (*corev1.GetUserResponseModel, error) {
	user, err := h.usersUC.GetUserByUsername(ctx, params.Username)
	if err != nil {
		return nil, err
	}

	return &corev1.GetUserResponseModel{
		User: userDTO(user),
	}, nil
}

func (h *CoreServer) GetMe(ctx context.Context) (*corev1.GetUserResponseModel, error) {
	user := middleware.GetUser(ctx)

	return &corev1.GetUserResponseModel{
		User: userDTO(user),
	}, nil
}

func (h *CoreServer) GetMyDashboard(ctx context.Context) (*corev1.GetUserDashboardResponseModel, error) {
	user := middleware.GetUser(ctx)

	contests, err := h.contestsUC.ListDashboardContests(ctx, user.Id, 5)
	if err != nil {
		return nil, err
	}

	problems, err := h.problemsUC.ListDashboardProblems(ctx, user.Id, 5)
	if err != nil {
		return nil, err
	}

	resp := DashboardResponseDTO(contests, problems)
	return &resp, nil
}

func (h *CoreServer) ListUsers(ctx context.Context, params corev1.ListUsersParams) (*corev1.ListUsersResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")
	role := params.Role.Or("")

	filter := models.UsersListFilter{
		Page:     page,
		PageSize: pageSize,
		Search:   search,
		Role:     role,
	}

	users, err := h.usersUC.ListUsers(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := usersListDTO(&users)
	return &resp, nil
}

func (h *CoreServer) ListUserSubmissions(ctx context.Context, params corev1.ListUserSubmissionsParams) (*corev1.ListSubmissionsResponseModel, error) {
	user, err := h.usersUC.GetUserByUsername(ctx, params.Username)
	if err != nil {
		return nil, err
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)

	var contestID *uuid.UUID
	if params.ContestId.IsSet() {
		contestID = &params.ContestId.Value
	}

	var problemID *uuid.UUID
	if params.ProblemId.IsSet() {
		problemID = &params.ProblemId.Value
	}

	var state *models.State
	if params.State.IsSet() {
		s := models.State(params.State.Value)
		state = &s
	}

	var order *int32
	if params.SortOrder.IsSet() {
		var orderVal int32
		if params.SortOrder.Value == corev1.ListUserSubmissionsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	filter := models.SubmissionsFilter{
		ContestId: contestID,
		Page:      page,
		PageSize:  pageSize,
		ProblemId: problemID,
		UserId:    &user.Id,
		Order:     order,
		State:     state,
	}

	submissions, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return SubmissionsListToDTO(submissions), nil
}

func (h *CoreServer) UpdateUser(ctx context.Context, req *corev1.UpdateUserRequestModel, params corev1.UpdateUserParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing body")
	}

	user, err := h.usersUC.GetUserByUsername(ctx, params.Username)
	if err != nil {
		return err
	}

	var reqUsername, reqEmail, reqRole *string
	if req.Username.IsSet() {
		reqUsername = &req.Username.Value
	}
	if req.Email.IsSet() {
		reqEmail = &req.Email.Value
	}
	if req.Role.IsSet() {
		r := string(req.Role.Value)
		reqRole = &r
	}

	input := models.UpdateUserInput{
		Id:       user.Id,
		Username: reqUsername,
		Email:    reqEmail,
		Role:     reqRole,
	}

	err = h.usersUC.UpdateUser(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ClaimTemporaryUser(ctx context.Context, req *corev1.ClaimTemporaryUserRequestModel) (*corev1.ClaimTemporaryUserResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing body")
	}

	user := middleware.GetUser(ctx)

	result, err := h.usersUC.ClaimTemporaryUser(ctx, user, models.ClaimTemporaryUserInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}

	return &corev1.ClaimTemporaryUserResponseModel{
		ClaimedUserID:   result.ClaimedUserID,
		ClaimedUsername: result.ClaimedUsername,
		ContestsGranted: result.ContestsGranted,
	}, nil
}

func (h *CoreServer) ListMyClaimedAccounts(ctx context.Context) (*corev1.ListClaimedAccountsResponseModel, error) {
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
			ID:        acc.ID,
			Username:  acc.Username,
			ClaimedAt: acc.ClaimedAt,
			ExpiresAt: timePtrToOptDateTime(acc.ExpiresAt),
		}
	}

	return &corev1.ListClaimedAccountsResponseModel{
		Accounts: items,
	}, nil
}

func (h *CoreServer) ChangePassword(ctx context.Context, req *corev1.ChangePasswordRequestModel) (*corev1.AuthResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	newSessionID, err := h.authUC.ChangePassword(ctx, user.Id, req.OldPassword, req.NewPassword)
	if err != nil {
		return nil, err
	}

	middleware.SetSessionCookie(ctx, newSessionID)

	return &corev1.AuthResponseModel{
		User:      userDTO(user),
		SessionID: corev1.NewOptUUID(newSessionID),
	}, nil
}

func (h *CoreServer) RequestEmailChange(ctx context.Context, req *corev1.RequestEmailChangeRequestModel) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.authUC.RequestEmailChange(ctx, user.Id, req.Password, req.NewEmail)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) AdminChangeEmail(ctx context.Context, req *corev1.AdminChangeEmailRequestModel, params corev1.AdminChangeEmailParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	withConfirmation := req.WithConfirmation.Or(true)

	err := h.usersUC.AdminChangeEmail(ctx, params.Username, req.Email, withConfirmation)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) AdminSetPassword(ctx context.Context, req *corev1.AdminSetPasswordRequestModel, params corev1.AdminSetPasswordParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.usersUC.AdminSetPassword(ctx, params.Username, req.Password)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) AdminSendPasswordReset(ctx context.Context, params corev1.AdminSendPasswordResetParams) error {
	err := h.usersUC.AdminSendPasswordReset(ctx, params.Username)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) AdminResendVerification(ctx context.Context, params corev1.AdminResendVerificationParams) error {
	err := h.usersUC.AdminResendVerification(ctx, params.Username)
	if err != nil {
		return err
	}

	return nil
}
