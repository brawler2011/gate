package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
)

// ListTeams handles GET /teams
func (h *CoreServer) ListTeams(ctx context.Context, request corev1.ListTeamsRequestObject) (corev1.ListTeamsResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Validate parameters
	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	err := validateListTeamsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	var teams []models.Team

	// If organization_id is specified, list teams for that organization
	if request.Params.OrganizationId != nil {
		teams, err = h.teamsUC.ListOrganizationTeams(ctx, *request.Params.OrganizationId, user.Id)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list organization teams")
		}
	} else {
		// Otherwise, list all teams the user is a member of
		teams, err = h.teamsUC.GetUserTeams(ctx, user.Id)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list user teams")
		}
	}

	// Apply search filter if provided (simple name filtering)
	if search != "" {
		filtered := make([]models.Team, 0)
		for _, team := range teams {
			if containsIgnoreCase(team.Name, search) {
				filtered = append(filtered, team)
			}
		}
		teams = filtered
	}

	// Calculate total
	total := int32(len(teams))

	// Apply pagination
	pageSize := int(request.Params.PageSize)
	page := int(request.Params.Page)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start > len(teams) {
		teams = []models.Team{}
	} else if end > len(teams) {
		teams = teams[start:]
	} else {
		teams = teams[start:end]
	}

	return corev1.ListTeams200JSONResponse(*listTeamsDTO(teams, request.Params.Page, total)), nil
}

// CreateTeam handles POST /teams
func (h *CoreServer) CreateTeam(ctx context.Context, request corev1.CreateTeamRequestObject) (corev1.CreateTeamResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Validate request body
	if err := validateCreateTeamRequest(request.Body.Name, request.Body.OrganizationId); err != nil {
		return nil, err
	}

	// Generate slug from name
	slug := generateLogin(request.Body.Name)

	// Create input
	input := &models.CreateTeamInput{
		OrganizationID: request.Body.OrganizationId,
		Name:           request.Body.Name,
		Slug:           slug,
		Description:    "",
		Privacy:        models.TeamPrivacyClosed, // Default privacy
		ParentTeamID:   nil,
	}

	// Create team
	team, err := h.teamsUC.CreateTeam(ctx, input, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to create team")
	}

	return corev1.CreateTeam200JSONResponse{
		Id: team.ID,
	}, nil
}

// GetTeam handles GET /teams/{id}
func (h *CoreServer) GetTeam(ctx context.Context, request corev1.GetTeamRequestObject) (corev1.GetTeamResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Get team
	team, err := h.teamsUC.GetTeam(ctx, request.Id, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get team")
	}

	return corev1.GetTeam200JSONResponse{
		Team: teamDTO(*team),
	}, nil
}

// UpdateTeam handles PATCH /teams/{id}
func (h *CoreServer) UpdateTeam(ctx context.Context, request corev1.UpdateTeamRequestObject) (corev1.UpdateTeamResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Validate request body
	if err := validateUpdateTeamRequest(*request.Body); err != nil {
		return nil, err
	}

	// Create update input
	input := &models.UpdateTeamInput{
		Name:        request.Body.Name,
		Description: request.Body.Description,
		Privacy:     nil, // Privacy not exposed in API yet
	}

	// Update team
	err := h.teamsUC.UpdateTeam(ctx, request.Id, user.Id, input)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to update team")
	}

	return corev1.UpdateTeam200Response{}, nil
}

// DeleteTeam handles DELETE /teams/{id}
func (h *CoreServer) DeleteTeam(ctx context.Context, request corev1.DeleteTeamRequestObject) (corev1.DeleteTeamResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Delete team
	err := h.teamsUC.DeleteTeam(ctx, request.Id, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to delete team")
	}

	return corev1.DeleteTeam200Response{}, nil
}

// ListTeamMembers handles GET /teams/{id}/members
func (h *CoreServer) ListTeamMembers(ctx context.Context, request corev1.ListTeamMembersRequestObject) (corev1.ListTeamMembersResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Validate parameters
	err := validateListTeamsParams(request.Params.Page, request.Params.PageSize, nil)
	if err != nil {
		return nil, err
	}

	// Get members
	members, err := h.teamsUC.ListTeamMembers(ctx, request.Id, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list team members")
	}

	// Calculate total for pagination (using actual count)
	total := int32(len(members))

	return corev1.ListTeamMembers200JSONResponse(*listTeamMembersDTO(members, request.Params.Page, total)), nil
}

// AddTeamMember handles POST /teams/{id}/members
func (h *CoreServer) AddTeamMember(ctx context.Context, request corev1.AddTeamMemberRequestObject) (corev1.AddTeamMemberResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Create input with default role "member"
	input := &models.AddTeamMemberInput{
		TeamID: request.Id,
		UserID: request.Params.UserId,
		Role:   models.TeamRoleMember, // Default role
	}

	// Add member
	err := h.teamsUC.AddTeamMember(ctx, input, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to add team member")
	}

	return corev1.AddTeamMember200Response{}, nil
}

// RemoveTeamMember handles DELETE /teams/{id}/members
func (h *CoreServer) RemoveTeamMember(ctx context.Context, request corev1.RemoveTeamMemberRequestObject) (corev1.RemoveTeamMemberResponseObject, error) {
	// Get current user
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Remove member
	err := h.teamsUC.RemoveTeamMember(ctx, request.Id, request.Params.UserId, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to remove team member")
	}

	return corev1.RemoveTeamMember200Response{}, nil
}

// containsIgnoreCase is a helper function to check if a string contains a substring (case-insensitive)
func containsIgnoreCase(str, substr string) bool {
	return len(str) >= len(substr) && 
		(substr == "" || 
		 len(substr) > 0 && 
		 indexIgnoreCase(str, substr) >= 0)
}

func indexIgnoreCase(str, substr string) int {
	strLower := toLower(str)
	substrLower := toLower(substr)
	for i := 0; i <= len(strLower)-len(substrLower); i++ {
		if strLower[i:i+len(substrLower)] == substrLower {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + ('a' - 'A')
		} else {
			result[i] = r
		}
	}
	return string(result)
}
