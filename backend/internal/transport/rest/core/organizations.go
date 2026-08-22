package core

import (
	"context"
	"errors"
	"regexp"
	"strings"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
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
func (h *CoreServer) ListOrganizations(ctx context.Context, request corev1.ListOrganizationsRequestObject) (corev1.ListOrganizationsResponseObject, error) {
	user := middleware.GetUser(ctx)

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	err := validateListOrganizationsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	filter := &models.OrganizationFilter{
		Search:   search,
		Page:     request.Params.Page,
		PageSize: request.Params.PageSize,
	}

	organizationsList, err := h.organizationsUC.ListOrganizations(ctx, filter, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list organizations")
	}

	return corev1.ListOrganizations200JSONResponse(*listOrganizationsDTO(organizationsList)), nil
}

// CreateOrganization handles POST /organizations
func (h *CoreServer) CreateOrganization(ctx context.Context, request corev1.CreateOrganizationRequestObject) (corev1.CreateOrganizationResponseObject, error) {
	user := middleware.GetUser(ctx)

	if err := validateCreateOrganizationParams(request.Params.Name); err != nil {
		return nil, err
	}

	var login string
	if request.Params.Login != nil && strings.TrimSpace(*request.Params.Login) != "" {
		login = strings.ToLower(strings.TrimSpace(*request.Params.Login))
		if err := validateOrgLogin(login); err != nil {
			return nil, err
		}
	} else {
		login = generateLogin(request.Params.Name)
		if login == "" {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "name must contain at least one latin letter or digit")
		}
		if err := validateOrgLogin(login); err != nil {
			return nil, err
		}
	}

	joinPolicy := models.OrgJoinPolicyByRequest
	if request.Params.JoinPolicy != nil {
		joinPolicy = models.OrganizationJoinPolicy(*request.Params.JoinPolicy)
	}

	input := &models.CreateOrganizationInput{
		Login:       login,
		Name:        request.Params.Name,
		Description: "",
		AvatarURL:   nil,
		JoinPolicy:  joinPolicy,
		CreatorID:   user.Id,
	}

	org, err := h.organizationsUC.CreateOrganization(ctx, input)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to create organization")
	}

	return corev1.CreateOrganization200JSONResponse{
		Id:    org.ID,
		Login: org.Login,
	}, nil
}

// GetOrganization handles GET /organizations/{login}
func (h *CoreServer) GetOrganization(ctx context.Context, request corev1.GetOrganizationRequestObject) (corev1.GetOrganizationResponseObject, error) {
	user := middleware.GetUser(ctx)

	org, err := h.organizationsUC.GetOrganizationByLogin(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to get organization")
	}

	return corev1.GetOrganization200JSONResponse{
		Organization: organizationDTO(*org),
	}, nil
}

// UpdateOrganization handles PATCH /organizations/{login}
func (h *CoreServer) UpdateOrganization(ctx context.Context, request corev1.UpdateOrganizationRequestObject) (corev1.UpdateOrganizationResponseObject, error) {
	user := middleware.GetUser(ctx)

	if err := validateUpdateOrganizationRequest(*request.Body); err != nil {
		return nil, err
	}

	var joinPolicy *models.OrganizationJoinPolicy
	if request.Body.JoinPolicy != nil {
		jp := models.OrganizationJoinPolicy(*request.Body.JoinPolicy)
		joinPolicy = &jp
	}

	input := &models.UpdateOrganizationInput{
		Login:       request.Body.Login,
		Name:        request.Body.Name,
		Description: request.Body.Description,
		AvatarURL:   nil,
		JoinPolicy:  joinPolicy,
	}

	err := h.organizationsUC.UpdateOrganizationByLogin(ctx, request.Login, user.Id, input)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to update organization")
	}

	return corev1.UpdateOrganization200Response{}, nil
}

// DeleteOrganization handles DELETE /organizations/{login}
func (h *CoreServer) DeleteOrganization(ctx context.Context, request corev1.DeleteOrganizationRequestObject) (corev1.DeleteOrganizationResponseObject, error) {
	user := middleware.GetUser(ctx)

	err := h.organizationsUC.DeleteOrganizationByLogin(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to delete organization")
	}

	return corev1.DeleteOrganization200Response{}, nil
}

// ListOrganizationMembers handles GET /organizations/{login}/members
func (h *CoreServer) ListOrganizationMembers(ctx context.Context, request corev1.ListOrganizationMembersRequestObject) (corev1.ListOrganizationMembersResponseObject, error) {
	user := middleware.GetUser(ctx)

	err := validateListOrganizationsParams(request.Params.Page, request.Params.PageSize, nil)
	if err != nil {
		return nil, err
	}

	members, err := h.organizationsUC.ListMembersByLogin(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list organization members")
	}

	total := safeInt32(len(members))
	return corev1.ListOrganizationMembers200JSONResponse(*listOrganizationMembersDTO(members, request.Params.Page, total)), nil
}

// AddOrganizationMember handles POST /organizations/{login}/members
func (h *CoreServer) AddOrganizationMember(ctx context.Context, request corev1.AddOrganizationMemberRequestObject) (corev1.AddOrganizationMemberResponseObject, error) {
	user := middleware.GetUser(ctx)

	if !validateOrganizationRole(request.Params.Role) {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid role, must be 'owner', 'admin', or 'member'")
	}

	input := &models.AddOrganizationMemberInput{
		UserID: request.Params.UserId,
		Role:   models.OrganizationRole(request.Params.Role),
	}

	err := h.organizationsUC.AddMemberByLogin(ctx, request.Login, input, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to add organization member")
	}

	return corev1.AddOrganizationMember200Response{}, nil
}

// RemoveOrganizationMember handles DELETE /organizations/{login}/members
func (h *CoreServer) RemoveOrganizationMember(ctx context.Context, request corev1.RemoveOrganizationMemberRequestObject) (corev1.RemoveOrganizationMemberResponseObject, error) {
	user := middleware.GetUser(ctx)

	err := h.organizationsUC.RemoveMemberByLogin(ctx, request.Login, request.Params.UserId, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to remove organization member")
	}

	return corev1.RemoveOrganizationMember200Response{}, nil
}

// BatchCreateOrganizationUsers handles POST /organizations/{login}/members/batch
func (h *CoreServer) BatchCreateOrganizationUsers(ctx context.Context, request corev1.BatchCreateOrganizationUsersRequestObject) (corev1.BatchCreateOrganizationUsersResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing body")
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	result, err := h.organizationsUC.BatchCreateUsers(ctx, models.BatchCreateOrganizationUsersInput{
		OrgLogin: request.Login,
		Prefix:   request.Body.Prefix,
		Count:    request.Body.Count,
		TTLDays:  request.Body.TtlDays,
	}, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to batch create users")
	}

	userItems := make([]corev1.BatchCreatedUserItem, len(result.Users))
	for i, u := range result.Users {
		userItems[i] = corev1.BatchCreatedUserItem{
			Id:        u.ID,
			Username:  u.Username,
			Password:  u.Password,
			ExpiresAt: u.ExpiresAt,
		}
	}

	return corev1.BatchCreateOrganizationUsers200JSONResponse{
		Users: userItems,
	}, nil
}

// Invitations

// ListOrganizationInvitations handles GET /organizations/{login}/invitations
func (h *CoreServer) ListOrganizationInvitations(ctx context.Context, request corev1.ListOrganizationInvitationsRequestObject) (corev1.ListOrganizationInvitationsResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	invs, err := h.organizationsUC.ListInvitations(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list invitations")
	}

	return corev1.ListOrganizationInvitations200JSONResponse(*ListOrganizationInvitationsResponseDTO(invs)), nil
}

// InviteOrganizationMember handles POST /organizations/{login}/invitations
func (h *CoreServer) InviteOrganizationMember(ctx context.Context, request corev1.InviteOrganizationMemberRequestObject) (corev1.InviteOrganizationMemberResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	role := models.OrganizationRole(request.Body.Role)
	if !validateOrganizationRole(string(role)) {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid role")
	}

	inv, err := h.organizationsUC.InviteMember(ctx, request.Login, request.Body.UserId, role, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to invite member")
	}

	return corev1.InviteOrganizationMember200JSONResponse(OrganizationInvitationDTO(*inv)), nil
}

// CancelOrganizationInvitation handles DELETE /organizations/{login}/invitations/{id}
func (h *CoreServer) CancelOrganizationInvitation(ctx context.Context, request corev1.CancelOrganizationInvitationRequestObject) (corev1.CancelOrganizationInvitationResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.organizationsUC.CancelInvitation(ctx, request.Login, request.Id, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to cancel invitation")
	}

	return corev1.CancelOrganizationInvitation200Response{}, nil
}

// AcceptOrganizationInvitation handles POST /invitations/{id}/accept
func (h *CoreServer) AcceptOrganizationInvitation(ctx context.Context, request corev1.AcceptOrganizationInvitationRequestObject) (corev1.AcceptOrganizationInvitationResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.organizationsUC.AcceptInvitation(ctx, request.Id, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to accept invitation")
	}

	return corev1.AcceptOrganizationInvitation200Response{}, nil
}

// DeclineOrganizationInvitation handles POST /invitations/{id}/decline
func (h *CoreServer) DeclineOrganizationInvitation(ctx context.Context, request corev1.DeclineOrganizationInvitationRequestObject) (corev1.DeclineOrganizationInvitationResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.organizationsUC.DeclineInvitation(ctx, request.Id, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to decline invitation")
	}

	return corev1.DeclineOrganizationInvitation200Response{}, nil
}

// Join Requests

// ListOrganizationJoinRequests handles GET /organizations/{login}/requests
func (h *CoreServer) ListOrganizationJoinRequests(ctx context.Context, request corev1.ListOrganizationJoinRequestsRequestObject) (corev1.ListOrganizationJoinRequestsResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	reqs, err := h.organizationsUC.ListJoinRequests(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list join requests")
	}

	return corev1.ListOrganizationJoinRequests200JSONResponse(*ListOrganizationJoinRequestsResponseDTO(reqs)), nil
}

// CreateOrganizationJoinRequest handles POST /organizations/{login}/requests
func (h *CoreServer) CreateOrganizationJoinRequest(ctx context.Context, request corev1.CreateOrganizationJoinRequestRequestObject) (corev1.CreateOrganizationJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	var message *string
	if request.Body != nil {
		message = request.Body.Message
	}

	req, joined, err := h.organizationsUC.CreateJoinRequest(ctx, request.Login, user.Id, message)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to create join request")
	}

	var reqDTO *corev1.OrganizationJoinRequestModel
	if req != nil {
		d := OrganizationJoinRequestDTO(*req)
		reqDTO = &d
	}

	return corev1.CreateOrganizationJoinRequest200JSONResponse{
		Joined:  joined,
		Request: reqDTO,
	}, nil
}

// GetMyOrganizationJoinRequest handles GET /organizations/{login}/requests/mine
func (h *CoreServer) GetMyOrganizationJoinRequest(ctx context.Context, request corev1.GetMyOrganizationJoinRequestRequestObject) (corev1.GetMyOrganizationJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return corev1.GetMyOrganizationJoinRequest200JSONResponse{Request: nil}, nil
	}

	req, err := h.organizationsUC.GetMyPendingJoinRequest(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to get join request")
	}

	var reqDTO *corev1.OrganizationJoinRequestModel
	if req != nil {
		d := OrganizationJoinRequestDTO(*req)
		reqDTO = &d
	}

	return corev1.GetMyOrganizationJoinRequest200JSONResponse{
		Request: reqDTO,
	}, nil
}

// CancelOrganizationJoinRequest handles DELETE /organizations/{login}/requests/mine
func (h *CoreServer) CancelOrganizationJoinRequest(ctx context.Context, request corev1.CancelOrganizationJoinRequestRequestObject) (corev1.CancelOrganizationJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.organizationsUC.CancelJoinRequest(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to cancel join request")
	}

	return corev1.CancelOrganizationJoinRequest200Response{}, nil
}

// ApproveOrganizationJoinRequest handles POST /organizations/{login}/requests/{id}/approve
func (h *CoreServer) ApproveOrganizationJoinRequest(ctx context.Context, request corev1.ApproveOrganizationJoinRequestRequestObject) (corev1.ApproveOrganizationJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	role := models.OrgRoleMember
	if request.Body != nil && request.Body.Role != nil {
		role = models.OrganizationRole(*request.Body.Role)
	}

	err := h.organizationsUC.ApproveJoinRequest(ctx, request.Login, request.Id, user.Id, role)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to approve join request")
	}

	return corev1.ApproveOrganizationJoinRequest200Response{}, nil
}

// RejectOrganizationJoinRequest handles POST /organizations/{login}/requests/{id}/reject
func (h *CoreServer) RejectOrganizationJoinRequest(ctx context.Context, request corev1.RejectOrganizationJoinRequestRequestObject) (corev1.RejectOrganizationJoinRequestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.organizationsUC.RejectJoinRequest(ctx, request.Login, request.Id, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to reject join request")
	}

	return corev1.RejectOrganizationJoinRequest200Response{}, nil
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
	return pkg.Wrap(pkg.ErrInternal, err, fallbackMsg)
}
