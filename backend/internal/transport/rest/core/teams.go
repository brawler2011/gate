package core

import (
	"context"
	"errors"
	"strings"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
)

// ListTeams handles GET /teams
func (h *CoreServer) ListTeams(ctx context.Context, params corev1.ListTeamsParams) (*corev1.ListTeamsResponseModel, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	page := params.Page
	pageSize := params.PageSize
	search := params.Search.Or("")

	var teams []models.Team
	var err error

	// If organization_id is specified, list teams for that organization
	if params.OrganizationID.IsSet() {
		teams, err = h.teamsUC.ListOrganizationTeams(ctx, params.OrganizationID.Value, user.Id)
		if err != nil {
			return nil, wrapTeamUCError(err, "failed to list organization teams")
		}
	} else {
		// Otherwise, list all teams the user is a member of
		teams, err = h.teamsUC.GetUserTeams(ctx, user.Id)
		if err != nil {
			return nil, wrapTeamUCError(err, "failed to list user teams")
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
	total := safeInt32(len(teams))

	// Apply pagination
	pSize := int(pageSize)
	pNum := int(page)
	start := (pNum - 1) * pSize
	end := start + pSize

	switch {
	case start > len(teams):
		teams = []models.Team{}
	case end > len(teams):
		teams = teams[start:]
	default:
		teams = teams[start:end]
	}

	return listTeamsDTO(teams, page, total), nil
}

// CreateTeam handles POST /teams
func (h *CoreServer) CreateTeam(ctx context.Context, req *corev1.CreateTeamReq) (*corev1.CreationResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	// Get current user
	user := middleware.GetUser(ctx)

	// Validate request body
	if err := validateCreateTeamRequest(req.Name, req.OrganizationID); err != nil {
		return nil, err
	}

	// Generate slug from name
	slug := generateLogin(req.Name)

	// Create input
	input := &models.CreateTeamInput{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Slug:           slug,
		Description:    "",
		Privacy:        models.TeamPrivacyClosed, // Default privacy
		ParentTeamID:   nil,
	}

	// Create team
	team, err := h.teamsUC.CreateTeam(ctx, input, user.Id)
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to create team")
	}

	return &corev1.CreationResponseModel{
		ID: team.ID,
	}, nil
}

// GetTeam handles GET /teams/{id}
func (h *CoreServer) GetTeam(ctx context.Context, params corev1.GetTeamParams) (*corev1.GetTeamResponseModel, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	// Get team
	team, err := h.teamsUC.GetTeam(ctx, params.ID, user.Id)
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to get team")
	}

	return &corev1.GetTeamResponseModel{
		Team: teamDTO(*team),
	}, nil
}

// UpdateTeam handles PATCH /teams/{id}
func (h *CoreServer) UpdateTeam(ctx context.Context, req *corev1.UpdateTeamRequestModel, params corev1.UpdateTeamParams) error {
	// Get current user
	user := middleware.GetUser(ctx)

	// Validate request body
	if err := validateUpdateTeamRequest(req); err != nil {
		return err
	}

	var reqName, reqDesc *string
	if req != nil {
		if req.Name.IsSet() {
			reqName = &req.Name.Value
		}
		if req.Description.IsSet() {
			reqDesc = &req.Description.Value
		}
	}

	// Create update input
	input := &models.UpdateTeamInput{
		Name:        reqName,
		Description: reqDesc,
		Privacy:     nil, // Privacy not exposed in API yet
	}

	// Update team
	err := h.teamsUC.UpdateTeam(ctx, params.ID, user.Id, input)
	if err != nil {
		return wrapTeamUCError(err, "failed to update team")
	}

	return nil
}

// DeleteTeam handles DELETE /teams/{id}
func (h *CoreServer) DeleteTeam(ctx context.Context, params corev1.DeleteTeamParams) error {
	// Get current user
	user := middleware.GetUser(ctx)

	// Delete team
	err := h.teamsUC.DeleteTeam(ctx, params.ID, user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to delete team")
	}

	return nil
}

// ListTeamMembers handles GET /teams/{id}/members
func (h *CoreServer) ListTeamMembers(ctx context.Context, params corev1.ListTeamMembersParams) (*corev1.ListTeamMembersResponseModel, error) {
	// Get current user
	user := middleware.GetUser(ctx)

	page := params.Page

	// Get members
	members, err := h.teamsUC.ListTeamMembers(ctx, params.ID, user.Id)
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to list team members")
	}

	// Calculate total for pagination (using actual count)
	total := safeInt32(len(members))

	return listTeamMembersDTO(members, page, total), nil
}

// AddTeamMember handles POST /teams/{id}/members
func (h *CoreServer) AddTeamMember(ctx context.Context, params corev1.AddTeamMemberParams) error {
	user := middleware.GetUser(ctx)

	role := models.TeamRoleMember
	if params.Role.IsSet() && params.Role.Value != "" {
		role = models.TeamRole(params.Role.Value)
	}

	input := &models.AddTeamMemberInput{
		TeamID: params.ID,
		UserID: params.UserID,
		Role:   role,
	}

	err := h.teamsUC.AddTeamMember(ctx, input, user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to add team member")
	}

	return nil
}

// UpdateTeamMemberRole handles PATCH /teams/{id}/members
func (h *CoreServer) UpdateTeamMemberRole(ctx context.Context, params corev1.UpdateTeamMemberRoleParams) error {
	user := middleware.GetUser(ctx)

	err := h.teamsUC.UpdateTeamMemberRole(ctx, params.ID, params.UserID, models.TeamRole(params.Role), user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to update team member role")
	}

	return nil
}

// RemoveTeamMember handles DELETE /teams/{id}/members
func (h *CoreServer) RemoveTeamMember(ctx context.Context, params corev1.RemoveTeamMemberParams) error {
	user := middleware.GetUser(ctx)

	err := h.teamsUC.RemoveTeamMember(ctx, params.ID, params.UserID, user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to remove team member")
	}

	return nil
}

// ListTeamContests handles GET /teams/{id}/contests
func (h *CoreServer) ListTeamContests(ctx context.Context, params corev1.ListTeamContestsParams) (*corev1.ListContestsResponseModel, error) {
	user := middleware.GetUser(ctx)

	contests, err := h.teamsUC.GetTeamContests(ctx, params.ID, user.Id)
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to list team contests")
	}

	contestsList := &models.ContestsList{
		Contests: contests,
		Pagination: models.Pagination{
			Page:  1,
			Total: safeInt32(len(contests)),
		},
	}

	return ListContestsResponseDTO(contestsList), nil
}

// ListTeamProblems handles GET /teams/{id}/problems
func (h *CoreServer) ListTeamProblems(ctx context.Context, params corev1.ListTeamProblemsParams) (*corev1.ListProblemsResponseModel, error) {
	user := middleware.GetUser(ctx)

	problems, err := h.teamsUC.GetTeamProblems(ctx, params.ID, user.Id)
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to list team problems")
	}

	problemsList := &models.ProblemsList{
		Problems: problems,
		Pagination: models.Pagination{
			Page:  1,
			Total: safeInt32(len(problems)),
		},
	}

	return ListProblemsResponseDTO(problemsList), nil
}

func wrapTeamUCError(err error, fallbackMsg string) error {
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
