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
	// Convert to lowercase
	login := strings.ToLower(name)
	// Replace spaces and special characters with hyphens
	login = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(login, "-")
	// Remove leading/trailing hyphens
	login = strings.Trim(login, "-")
	// Collapse multiple hyphens
	login = regexp.MustCompile(`-+`).ReplaceAllString(login, "-")
	return login
}

// ListOrganizations handles GET /organizations
func (h *CoreServer) ListOrganizations(ctx context.Context, request corev1.ListOrganizationsRequestObject) (corev1.ListOrganizationsResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate parameters
	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	err := validateListOrganizationsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	// Create filter
	filter := &models.OrganizationFilter{
		Search:   search,
		Page:     request.Params.Page,
		PageSize: request.Params.PageSize,
	}

	// Get organizations
	organizationsList, err := h.organizationsUC.ListOrganizations(ctx, filter, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list organizations")
	}

	return corev1.ListOrganizations200JSONResponse(*listOrganizationsDTO(organizationsList)), nil
}

// CreateOrganization handles POST /organizations
func (h *CoreServer) CreateOrganization(ctx context.Context, request corev1.CreateOrganizationRequestObject) (corev1.CreateOrganizationResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate parameters
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
		// Generate login from name
		login = generateLogin(request.Params.Name)
		if login == "" {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "name must contain at least one latin letter or digit")
		}
		if err := validateOrgLogin(login); err != nil {
			return nil, err
		}
	}

	// Create input
	input := &models.CreateOrganizationInput{
		Login:       login,
		Name:        request.Params.Name,
		Description: "",
		AvatarURL:   nil,
		CreatorID:   user.Id,
	}

	// Create organization
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
	// Get current user
	user := middleware.GetUser(ctx)

	// Get organization
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
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate request body
	if err := validateUpdateOrganizationRequest(*request.Body); err != nil {
		return nil, err
	}

	// Create update input
	input := &models.UpdateOrganizationInput{
		Login:       request.Body.Login,
		Name:        request.Body.Name,
		Description: request.Body.Description,
		AvatarURL:   nil,
	}

	// Update organization
	err := h.organizationsUC.UpdateOrganizationByLogin(ctx, request.Login, user.Id, input)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to update organization")
	}

	return corev1.UpdateOrganization200Response{}, nil
}

// DeleteOrganization handles DELETE /organizations/{login}
func (h *CoreServer) DeleteOrganization(ctx context.Context, request corev1.DeleteOrganizationRequestObject) (corev1.DeleteOrganizationResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Delete organization
	err := h.organizationsUC.DeleteOrganizationByLogin(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to delete organization")
	}

	return corev1.DeleteOrganization200Response{}, nil
}

// ListOrganizationMembers handles GET /organizations/{login}/members
func (h *CoreServer) ListOrganizationMembers(ctx context.Context, request corev1.ListOrganizationMembersRequestObject) (corev1.ListOrganizationMembersResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate parameters
	err := validateListOrganizationsParams(request.Params.Page, request.Params.PageSize, nil)
	if err != nil {
		return nil, err
	}

	// Get members
	members, err := h.organizationsUC.ListMembersByLogin(ctx, request.Login, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to list organization members")
	}

	// Calculate total for pagination (using actual count)
	total := safeInt32(len(members))

	return corev1.ListOrganizationMembers200JSONResponse(*listOrganizationMembersDTO(members, request.Params.Page, total)), nil
}

// AddOrganizationMember handles POST /organizations/{login}/members
func (h *CoreServer) AddOrganizationMember(ctx context.Context, request corev1.AddOrganizationMemberRequestObject) (corev1.AddOrganizationMemberResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate role
	if !validateOrganizationRole(request.Params.Role) {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid role, must be 'owner', 'admin', or 'member'")
	}

	// Create input
	input := &models.AddOrganizationMemberInput{
		UserID: request.Params.UserId,
		Role:   models.OrganizationRole(request.Params.Role),
	}

	// Add member
	err := h.organizationsUC.AddMemberByLogin(ctx, request.Login, input, user.Id)
	if err != nil {
		return nil, wrapOrgUCError(err, "failed to add organization member")
	}

	return corev1.AddOrganizationMember200Response{}, nil
}

// RemoveOrganizationMember handles DELETE /organizations/{login}/members
func (h *CoreServer) RemoveOrganizationMember(ctx context.Context, request corev1.RemoveOrganizationMemberRequestObject) (corev1.RemoveOrganizationMemberResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Remove member
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
	return pkg.Wrap(pkg.ErrInternal, err, fallbackMsg)
}
