package core

import (
	"context"
	"regexp"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	paging "github.com/brawler2011/gate/backend/internal/page"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
)

func generateLogin(name string) string {
	login := strings.ToLower(name)
	login = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(login, "-")
	login = strings.Trim(login, "-")
	login = regexp.MustCompile(`-+`).ReplaceAllString(login, "-")
	return login
}

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
		return nil, err
	}

	return projectResponse(h, ctx, listOrganizationsDTO(organizationsList))
}

func (h *CoreServer) CreateOrganization(ctx context.Context, params corev1.CreateOrganizationParams) (*corev1.CreateOrganizationResponseModel, error) {
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
			return nil, pkg.AtField("/name", pkg.Invalid(pkg.ErrFormat, "name must contain at least one latin letter or digit", nil))
		}
		if err := validateOrgLogin(login); err != nil {
			return nil, err
		}
	}

	joinPolicy := accessmodel.JoinByRequest
	if params.JoinPolicy.IsSet() {
		joinPolicy = accessmodel.JoinPolicy(params.JoinPolicy.Value)
	}

	input := &organizationsmodel.CreateOrganizationInput{
		Login:       login,
		Name:        params.Name,
		Description: "",
		AvatarURL:   nil,
		JoinPolicy:  joinPolicy,
	}

	org, err := h.Organizations.CreateOrganization(ctx, input)
	if err != nil {
		return nil, err
	}

	return &corev1.CreateOrganizationResponseModel{
		ID:    org.ID,
		Login: org.Login,
	}, nil
}

func (h *CoreServer) GetOrganization(ctx context.Context, params corev1.GetOrganizationParams) (*corev1.GetOrganizationResponseModel, error) {
	org, err := h.Organizations.GetOrganizationByLogin(ctx, params.Login)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, &corev1.GetOrganizationResponseModel{
		Organization: organizationDTO(*org),
	})
}

func (h *CoreServer) UpdateOrganization(ctx context.Context, req *corev1.UpdateOrganizationRequestModel, params corev1.UpdateOrganizationParams) error {
	if err := validateUpdateOrganizationRequest(req); err != nil {
		return err
	}

	var joinPolicy *accessmodel.JoinPolicy
	if req.JoinPolicy.IsSet() {
		jp := accessmodel.JoinPolicy(req.JoinPolicy.Value)
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
		return err
	}

	return nil
}

func (h *CoreServer) DeleteOrganization(ctx context.Context, params corev1.DeleteOrganizationParams) error {
	err := h.Organizations.DeleteOrganizationByLogin(ctx, params.Login)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListOrganizationMembers(ctx context.Context, params corev1.ListOrganizationMembersParams) (*corev1.ListOrganizationMembersResponseModel, error) {
	members, err := h.Organizations.ListMembersByLogin(ctx, params.Login, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, listOrganizationMembersDTO(members.Items, members.Pagination))
}

func (h *CoreServer) AddOrganizationMember(ctx context.Context, params corev1.AddOrganizationMemberParams) error {
	if !validateOrganizationRole(string(params.Role)) {
		return pkg.Wrap(pkg.ErrInvalid, nil, "invalid role, must be 'owner', 'admin', or 'member'")
	}

	input := &organizationsmodel.AddOrganizationMemberInput{
		UserID: params.UserID,
		Role:   accessmodel.OrganizationRole(params.Role),
	}

	err := h.Organizations.AddMemberByLogin(ctx, params.Login, input)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) RemoveOrganizationMember(ctx context.Context, params corev1.RemoveOrganizationMemberParams) error {
	err := h.Organizations.RemoveMemberByLogin(ctx, params.Login, params.UserID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) BatchCreateOrganizationUsers(ctx context.Context, req *corev1.BatchCreateOrganizationUsersRequestModel, params corev1.BatchCreateOrganizationUsersParams) (*corev1.BatchCreateOrganizationUsersResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "missing body")
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
		return nil, err
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

func (h *CoreServer) ListOrganizationInvitations(ctx context.Context, params corev1.ListOrganizationInvitationsParams) (*corev1.ListOrganizationInvitationsResponseModel, error) {
	invs, err := h.Organizations.ListInvitations(ctx, params.Login)
	if err != nil {
		return nil, err
	}

	return ListOrganizationInvitationsResponseDTO(invs), nil
}

func (h *CoreServer) InviteOrganizationMember(ctx context.Context, req *corev1.InviteOrganizationMemberRequestModel, params corev1.InviteOrganizationMemberParams) (*corev1.OrganizationInvitationModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "missing request body")
	}

	role := accessmodel.OrganizationRole(req.Role)
	if !validateOrganizationRole(string(role)) {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "invalid role")
	}

	inv, err := h.Organizations.InviteMember(ctx, params.Login, req.UserID, role)
	if err != nil {
		return nil, err
	}

	res := OrganizationInvitationDTO(*inv)
	return &res, nil
}

func (h *CoreServer) CancelOrganizationInvitation(ctx context.Context, params corev1.CancelOrganizationInvitationParams) error {
	err := h.Organizations.CancelInvitation(ctx, params.Login, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) AcceptOrganizationInvitation(ctx context.Context, params corev1.AcceptOrganizationInvitationParams) error {

	err := h.Organizations.AcceptInvitation(ctx, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeclineOrganizationInvitation(ctx context.Context, params corev1.DeclineOrganizationInvitationParams) error {

	err := h.Organizations.DeclineInvitation(ctx, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListOrganizationJoinRequests(ctx context.Context, params corev1.ListOrganizationJoinRequestsParams) (*corev1.ListOrganizationJoinRequestsResponseModel, error) {
	reqs, err := h.Organizations.ListJoinRequests(ctx, params.Login)
	if err != nil {
		return nil, err
	}

	return ListOrganizationJoinRequestsResponseDTO(reqs), nil
}

func (h *CoreServer) CreateOrganizationJoinRequest(ctx context.Context, req corev1.OptCreateOrganizationJoinRequestModel, params corev1.CreateOrganizationJoinRequestParams) (*corev1.OrganizationJoinRequestResponseModel, error) {

	var message *string
	if req.IsSet() && req.Value.Message.IsSet() {
		message = &req.Value.Message.Value
	}

	joinReq, joined, err := h.Organizations.CreateJoinRequest(ctx, params.Login, message)
	if err != nil {
		return nil, err
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

func (h *CoreServer) GetMyOrganizationJoinRequest(ctx context.Context, params corev1.GetMyOrganizationJoinRequestParams) (*corev1.OrganizationJoinRequestNullableResponseModel, error) {
	joinReq, err := h.Organizations.GetMyPendingJoinRequest(ctx, params.Login)
	if err != nil {
		return nil, err
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

func (h *CoreServer) CancelOrganizationJoinRequest(ctx context.Context, params corev1.CancelOrganizationJoinRequestParams) error {

	err := h.Organizations.CancelJoinRequest(ctx, params.Login)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ApproveOrganizationJoinRequest(ctx context.Context, req corev1.OptApproveOrganizationJoinRequestModel, params corev1.ApproveOrganizationJoinRequestParams) error {
	role := accessmodel.OrganizationMember
	if req.IsSet() && req.Value.Role.IsSet() {
		role = accessmodel.OrganizationRole(req.Value.Role.Value)
		if !validateOrganizationRole(string(role)) {
			return pkg.Wrap(pkg.ErrInvalid, nil, "invalid role")
		}
	}

	err := h.Organizations.ApproveJoinRequest(ctx, params.Login, params.ID, role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) RejectOrganizationJoinRequest(ctx context.Context, params corev1.RejectOrganizationJoinRequestParams) error {
	err := h.Organizations.RejectJoinRequest(ctx, params.Login, params.ID)
	if err != nil {
		return err
	}

	return nil
}
