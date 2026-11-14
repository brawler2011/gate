package core

import (
	"context"
	"errors"
	"regexp"
	"strings"

	paging "github.com/brawler2011/gate/backend/internal/page"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
)

// generateLogin creates a URL-safe login from a name
func generateLogin(name string) string {
	login := strings.ToLower(name)
	login = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(login, "-")
	login = strings.Trim(login, "-")
	login = regexp.MustCompile(`-+`).ReplaceAllString(login, "-")
	return login
}

// ListOrganizations handles GET /organizations
func (h *CoreServer) ListOrganizations(ctx context.Context, params corev1.ListOrganizationsParams) (*corev1.ListOrganizationsResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)
	search := params.Search.Or("")

	filter := &organizationsmodel.OrganizationFilter{
		Search:   search,
		Page:     page,
		PageSize: pageSize,
	}

	organizationsList, err := h.Organizations.ListOrganizations(ctx, filter)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list organizations")
	}

	return projectResponse(h, ctx, listOrganizationsDTO(organizationsList))
}

// CreateOrganization handles POST /organizations
func (h *CoreServer) CreateOrganization(ctx context.Context, params corev1.CreateOrganizationParams) (*corev1.CreateOrganizationResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)

	if err := validateCreateOrganizationParams(params.Name); err != nil {
		return nil, err
	}

	var login string
	if params.Login.IsSet() && strings.TrimSpace(params.Login.Value) != "" {
		login = strings.ToLower(strings.TrimSpace(params.Login.Value))
		if err := validateOrgLogin(login); err != nil {
			return nil, err
		}
	} else {
		login = generateLogin(params.Name)
		if login == "" {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "name must contain at least one latin letter or digit")
		}
		if err := validateOrgLogin(login); err != nil {
			return nil, err
		}
	}

	joinPolicy := organizationsmodel.OrgJoinPolicyByRequest
	if params.JoinPolicy.IsSet() {
		joinPolicy = organizationsmodel.OrganizationJoinPolicy(params.JoinPolicy.Value)
	}

	input := &organizationsmodel.CreateOrganizationInput{
		Login:       login,
		Name:        params.Name,
		Description: "",
		AvatarURL:   nil,
		JoinPolicy:  joinPolicy,
		CreatorID:   user.Id,
	}

	org, err := h.Organizations.CreateOrganization(ctx, input)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to create organization")
	}

	return &corev1.CreateOrganizationResponseModel{
		ID:    org.ID,
		Login: org.Login,
	}, nil
}

// GetOrganization handles GET /organizations/{login}
func (h *CoreServer) GetOrganization(ctx context.Context, params corev1.GetOrganizationParams) (*corev1.GetOrganizationResponseModel, error) {
	org, err := h.Organizations.GetOrganizationByLogin(ctx, params.Login)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to get organization")
	}

	return projectResponse(h, ctx, &corev1.GetOrganizationResponseModel{
		Organization: organizationDTO(*org),
	})
}

// UpdateOrganization handles PATCH /organizations/{login}
func (h *CoreServer) UpdateOrganization(ctx context.Context, req *corev1.UpdateOrganizationRequestModel, params corev1.UpdateOrganizationParams) error {
	if err := validateUpdateOrganizationRequest(req); err != nil {
		return err
	}

	var joinPolicy *organizationsmodel.OrganizationJoinPolicy
	if req.JoinPolicy.IsSet() {
		jp := organizationsmodel.OrganizationJoinPolicy(req.JoinPolicy.Value)
		joinPolicy = &jp
	}

	var reqLogin, reqName, reqDesc *string
	if req.Login.IsSet() {
		reqLogin = &req.Login.Value
	}
	if req.Name.IsSet() {
		reqName = &req.Name.Value
	}
	if req.Description.IsSet() {
		reqDesc = &req.Description.Value
	}

	input := &organizationsmodel.UpdateOrganizationInput{
		Login:       reqLogin,
		Name:        reqName,
		Description: reqDesc,
		AvatarURL:   nil,
		JoinPolicy:  joinPolicy,
	}

	err := h.Organizations.UpdateOrganizationByLogin(ctx, params.Login, input)
	if err != nil {
		return wrapOrgUCError(err, "failed to update organization")
	}

	return nil
}

// DeleteOrganization handles DELETE /organizations/{login}
func (h *CoreServer) DeleteOrganization(ctx context.Context, params corev1.DeleteOrganizationParams) error {
	err := h.Organizations.DeleteOrganizationByLogin(ctx, params.Login)
	if err != nil {
		return wrapOrgUCError(err, "failed to delete organization")
	}

	return nil
}

// ListOrganizationMembers handles GET /organizations/{login}/members
func (h *CoreServer) ListOrganizationMembers(ctx context.Context, params corev1.ListOrganizationMembersParams) (*corev1.ListOrganizationMembersResponseModel, error) {
	members, err := h.Organizations.ListMembersByLogin(ctx, params.Login, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list organization members")
	}

	return projectResponse(h, ctx, listOrganizationMembersDTO(members.Items, members.Pagination))
}

// AddOrganizationMember handles POST /organizations/{login}/members
func (h *CoreServer) AddOrganizationMember(ctx context.Context, params corev1.AddOrganizationMemberParams) error {
	if !validateOrganizationRole(string(params.Role)) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role, must be 'owner', 'admin', or 'member'")
	}

	input := &organizationsmodel.AddOrganizationMemberInput{
		UserID: params.UserID,
		Role:   organizationsmodel.OrganizationRole(params.Role),
	}

	err := h.Organizations.AddMemberByLogin(ctx, params.Login, input)
	if err != nil {
		return wrapOrgUCError(err, "failed to add organization member")
	}

	return nil
}

// RemoveOrganizationMember handles DELETE /organizations/{login}/members
func (h *CoreServer) RemoveOrganizationMember(ctx context.Context, params corev1.RemoveOrganizationMemberParams) error {
	err := h.Organizations.RemoveMemberByLogin(ctx, params.Login, params.UserID)
	if err != nil {
		return wrapOrgUCError(err, "failed to remove organization member")
	}

	return nil
}

// BatchCreateOrganizationUsers handles POST /organizations/{login}/members/batch
func (h *CoreServer) BatchCreateOrganizationUsers(ctx context.Context, req *corev1.BatchCreateOrganizationUsersRequestModel, params corev1.BatchCreateOrganizationUsersParams) (*corev1.BatchCreateOrganizationUsersResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing body")
	}

	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	var ttlDays *int32
	if req.TTLDays.IsSet() {
		ttlDays = &req.TTLDays.Value
	}

	result, err := h.Organizations.BatchCreateUsers(ctx, organizationsmodel.BatchCreateOrganizationUsersInput{
		OrgLogin: params.Login,
		Prefix:   req.Prefix,
		Count:    req.Count,
		TTLDays:  ttlDays,
	})
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to batch create users")
	}

	userItems := make([]corev1.BatchCreatedUserItem, len(result.Users))
	for i, u := range result.Users {
		userItems[i] = corev1.BatchCreatedUserItem{
			ID:        u.ID,
			Username:  u.Username,
			Password:  u.Password,
			ExpiresAt: timePtrToOptDateTime(u.ExpiresAt),
		}
	}

	return &corev1.BatchCreateOrganizationUsersResponseModel{
		Users: userItems,
	}, nil
}

// ListOrganizationInvitations handles GET /organizations/{login}/invitations
func (h *CoreServer) ListOrganizationInvitations(ctx context.Context, params corev1.ListOrganizationInvitationsParams) (*corev1.ListOrganizationInvitationsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	invs, err := h.Organizations.ListInvitations(ctx, params.Login)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list invitations")
	}

	return ListOrganizationInvitationsResponseDTO(invs), nil
}

// InviteOrganizationMember handles POST /organizations/{login}/invitations
func (h *CoreServer) InviteOrganizationMember(ctx context.Context, req *corev1.InviteOrganizationMemberRequestModel, params corev1.InviteOrganizationMemberParams) (*corev1.OrganizationInvitationModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	role := organizationsmodel.OrganizationRole(req.Role)
	if !validateOrganizationRole(string(role)) {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid role")
	}

	inv, err := h.Organizations.InviteMember(ctx, params.Login, req.UserID, role)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to invite member")
	}

	res := OrganizationInvitationDTO(*inv)
	return &res, nil
}

// CancelOrganizationInvitation handles DELETE /organizations/{login}/invitations/{id}
func (h *CoreServer) CancelOrganizationInvitation(ctx context.Context, params corev1.CancelOrganizationInvitationParams) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.Organizations.CancelInvitation(ctx, params.Login, params.ID)
	if err != nil {
		return wrapOrgUCError(err, "failed to cancel invitation")
	}

	return nil
}

// AcceptOrganizationInvitation handles POST /invitations/{id}/accept
func (h *CoreServer) AcceptOrganizationInvitation(ctx context.Context, params corev1.AcceptOrganizationInvitationParams) error {

	err := h.Organizations.AcceptInvitation(ctx, params.ID)
	if err != nil {
		return wrapOrgUCError(err, "failed to accept invitation")
	}

	return nil
}

// DeclineOrganizationInvitation handles POST /invitations/{id}/decline
func (h *CoreServer) DeclineOrganizationInvitation(ctx context.Context, params corev1.DeclineOrganizationInvitationParams) error {

	err := h.Organizations.DeclineInvitation(ctx, params.ID)
	if err != nil {
		return wrapOrgUCError(err, "failed to decline invitation")
	}

	return nil
}

// ListOrganizationJoinRequests handles GET /organizations/{login}/requests
func (h *CoreServer) ListOrganizationJoinRequests(ctx context.Context, params corev1.ListOrganizationJoinRequestsParams) (*corev1.ListOrganizationJoinRequestsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	reqs, err := h.Organizations.ListJoinRequests(ctx, params.Login)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list join requests")
	}

	return ListOrganizationJoinRequestsResponseDTO(reqs), nil
}

// CreateOrganizationJoinRequest handles POST /organizations/{login}/requests
func (h *CoreServer) CreateOrganizationJoinRequest(ctx context.Context, req corev1.OptCreateOrganizationJoinRequestModel, params corev1.CreateOrganizationJoinRequestParams) (*corev1.OrganizationJoinRequestResponseModel, error) {

	var message *string
	if req.IsSet() && req.Value.Message.IsSet() {
		message = &req.Value.Message.Value
	}

	joinReq, joined, err := h.Organizations.CreateJoinRequest(ctx, params.Login, message)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to create join request")
	}

	var reqOpt corev1.OptOrganizationJoinRequestModel
	if joinReq != nil {
		d := OrganizationJoinRequestDTO(*joinReq)
		reqOpt = corev1.NewOptOrganizationJoinRequestModel(d)
	}

	return &corev1.OrganizationJoinRequestResponseModel{
		Joined:  joined,
		Request: reqOpt,
	}, nil
}

// GetMyOrganizationJoinRequest handles GET /organizations/{login}/requests/mine
func (h *CoreServer) GetMyOrganizationJoinRequest(ctx context.Context, params corev1.GetMyOrganizationJoinRequestParams) (*corev1.OrganizationJoinRequestNullableResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return &corev1.OrganizationJoinRequestNullableResponseModel{}, nil
	}

	joinReq, err := h.Organizations.GetMyPendingJoinRequest(ctx, params.Login)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to get join request")
	}

	var reqOpt corev1.OptOrganizationJoinRequestModel
	if joinReq != nil {
		d := OrganizationJoinRequestDTO(*joinReq)
		reqOpt = corev1.NewOptOrganizationJoinRequestModel(d)
	}

	return &corev1.OrganizationJoinRequestNullableResponseModel{
		Request: reqOpt,
	}, nil
}

// CancelOrganizationJoinRequest handles DELETE /organizations/{login}/requests/mine
func (h *CoreServer) CancelOrganizationJoinRequest(ctx context.Context, params corev1.CancelOrganizationJoinRequestParams) error {

	err := h.Organizations.CancelJoinRequest(ctx, params.Login)
	if err != nil {
		return wrapOrgUCError(err, "failed to cancel join request")
	}

	return nil
}

// ApproveOrganizationJoinRequest handles POST /organizations/{login}/requests/{id}/approve
func (h *CoreServer) ApproveOrganizationJoinRequest(ctx context.Context, req corev1.OptApproveOrganizationJoinRequestModel, params corev1.ApproveOrganizationJoinRequestParams) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	role := organizationsmodel.OrgRoleMember
	if req.IsSet() && req.Value.Role.IsSet() {
		role = organizationsmodel.OrganizationRole(req.Value.Role.Value)
		if !validateOrganizationRole(string(role)) {
			return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role")
		}
	}

	err := h.Organizations.ApproveJoinRequest(ctx, params.Login, params.ID, role)
	if err != nil {
		return wrapOrgUCError(err, "failed to approve join request")
	}

	return nil
}

// RejectOrganizationJoinRequest handles POST /organizations/{login}/requests/{id}/reject
func (h *CoreServer) RejectOrganizationJoinRequest(ctx context.Context, params corev1.RejectOrganizationJoinRequestParams) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.Organizations.RejectJoinRequest(ctx, params.Login, params.ID)
	if err != nil {
		return wrapOrgUCError(err, "failed to reject join request")
	}

	return nil
}

func wrapOrgUCError(err error, fallbackMsg string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pkg.NoPermission) || strings.Contains(err.Error(), "access denied") {
		return pkg.Wrap(pkg.NoPermission, err, fallbackMsg)
	}
	if errors.Is(err, pkg.ErrNotFound) {
		return pkg.Wrap(pkg.ErrNotFound, err, fallbackMsg)
	}
	if errors.Is(err, pkg.ErrBadInput) {
		return pkg.Wrap(pkg.ErrBadInput, err, fallbackMsg)
	}
	// Adding context must not change the use case error classification.
	// Unclassified failures still map to HTTP 500 in pkg.ToREST.
	return pkg.Wrap(nil, err, fallbackMsg)
}
