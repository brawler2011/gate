package core

import (
	"context"
	"regexp"
	"strings"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
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
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list organizations")
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

	// Generate login from name
	login := generateLogin(request.Params.Name)
	if login == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "name must contain at least one latin letter or digit")
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
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to create organization")
	}

	return corev1.CreateOrganization200JSONResponse{
		Id: org.ID,
	}, nil
}

// GetOrganization handles GET /organizations/{id}
func (h *CoreServer) GetOrganization(ctx context.Context, request corev1.GetOrganizationRequestObject) (corev1.GetOrganizationResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Get organization
	org, err := h.organizationsUC.GetOrganization(ctx, request.Id, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get organization")
	}

	return corev1.GetOrganization200JSONResponse{
		Organization: organizationDTO(*org),
	}, nil
}

// UpdateOrganization handles PATCH /organizations/{id}
func (h *CoreServer) UpdateOrganization(ctx context.Context, request corev1.UpdateOrganizationRequestObject) (corev1.UpdateOrganizationResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate request body
	if err := validateUpdateOrganizationRequest(*request.Body); err != nil {
		return nil, err
	}

	// Create update input
	input := &models.UpdateOrganizationInput{
		Name:        request.Body.Name,
		Description: request.Body.Description,
		AvatarURL:   nil,
	}

	// Update organization
	err := h.organizationsUC.UpdateOrganization(ctx, request.Id, user.Id, input)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to update organization")
	}

	return corev1.UpdateOrganization200Response{}, nil
}

// DeleteOrganization handles DELETE /organizations/{id}
func (h *CoreServer) DeleteOrganization(ctx context.Context, request corev1.DeleteOrganizationRequestObject) (corev1.DeleteOrganizationResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Delete organization
	err := h.organizationsUC.DeleteOrganization(ctx, request.Id, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to delete organization")
	}

	return corev1.DeleteOrganization200Response{}, nil
}

// ListOrganizationMembers handles GET /organizations/{id}/members
func (h *CoreServer) ListOrganizationMembers(ctx context.Context, request corev1.ListOrganizationMembersRequestObject) (corev1.ListOrganizationMembersResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate parameters
	err := validateListOrganizationsParams(request.Params.Page, request.Params.PageSize, nil)
	if err != nil {
		return nil, err
	}

	// Get members
	members, err := h.organizationsUC.ListMembers(ctx, request.Id, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list organization members")
	}

	// Calculate total for pagination (using actual count)
	total := int32(len(members))

	return corev1.ListOrganizationMembers200JSONResponse(*listOrganizationMembersDTO(members, request.Params.Page, total)), nil
}

// AddOrganizationMember handles POST /organizations/{id}/members
func (h *CoreServer) AddOrganizationMember(ctx context.Context, request corev1.AddOrganizationMemberRequestObject) (corev1.AddOrganizationMemberResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate role
	if !validateOrganizationRole(request.Params.Role) {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid role, must be 'owner', 'admin', or 'member'")
	}

	// Create input
	input := &models.AddOrganizationMemberInput{
		OrganizationID: request.Id,
		UserID:         request.Params.UserId,
		Role:           models.OrganizationRole(request.Params.Role),
	}

	// Add member
	err := h.organizationsUC.AddMember(ctx, input, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to add organization member")
	}

	return corev1.AddOrganizationMember200Response{}, nil
}

// RemoveOrganizationMember handles DELETE /organizations/{id}/members
func (h *CoreServer) RemoveOrganizationMember(ctx context.Context, request corev1.RemoveOrganizationMemberRequestObject) (corev1.RemoveOrganizationMemberResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Remove member
	err := h.organizationsUC.RemoveMember(ctx, request.Id, request.Params.UserId, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to remove organization member")
	}

	return corev1.RemoveOrganizationMember200Response{}, nil
}
