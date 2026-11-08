package core

import (
	"context"
	"errors"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
)

// ListTeams handles GET /teams
func (h *CoreServer) ListTeams(ctx context.Context, params corev1.ListTeamsParams) (*corev1.ListTeamsResponseModel, error) {
	filter := &organizationsmodel.TeamsFilter{
		Search:   params.Search.Or(""),
		Page:     params.Page.Or(1),
		PageSize: params.PageSize.Or(50),
	}
	if params.OrganizationID.IsSet() {
		filter.OrganizationID = &params.OrganizationID.Value
	}
	result, err := h.organizations.ListTeams(ctx, filter)
	if err != nil {
		return nil, err
	}
	return projectResponse(h, ctx, listTeamsDTO(result.Teams, result.Pagination))
}

// CreateTeam handles POST /teams
func (h *CoreServer) CreateTeam(ctx context.Context, req *corev1.CreateTeamReq) (*corev1.CreationResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	// Get current user
	user := accessmodel.UserFromContext(ctx)

	// Validate request body
	if err := validateCreateTeamRequest(req.Name, req.OrganizationID); err != nil {
		return nil, err
	}

	// Generate slug from name
	slug := generateLogin(req.Name)

	// Create input
	input := &organizationsmodel.CreateTeamInput{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Slug:           slug,
		Description:    "",
		Privacy:        organizationsmodel.TeamPrivacyClosed, // Default privacy
		ParentTeamID:   nil,
	}

	// Create team
	team, err := h.organizations.CreateTeam(ctx, input, user.Id)
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
	user := accessmodel.UserFromContext(ctx)

	// Get team
	team, err := h.organizations.GetTeam(ctx, params.ID, user.Id)
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to get team")
	}

	return projectResponse(h, ctx, &corev1.GetTeamResponseModel{
		Team: teamDTO(*team),
	})
}

// UpdateTeam handles PATCH /teams/{id}
func (h *CoreServer) UpdateTeam(ctx context.Context, req *corev1.UpdateTeamRequestModel, params corev1.UpdateTeamParams) error {
	// Get current user
	user := accessmodel.UserFromContext(ctx)

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
	input := &organizationsmodel.UpdateTeamInput{
		Name:        reqName,
		Description: reqDesc,
		Privacy:     nil, // Privacy not exposed in API yet
	}

	// Update team
	err := h.organizations.UpdateTeam(ctx, params.ID, user.Id, input)
	if err != nil {
		return wrapTeamUCError(err, "failed to update team")
	}

	return nil
}

// DeleteTeam handles DELETE /teams/{id}
func (h *CoreServer) DeleteTeam(ctx context.Context, params corev1.DeleteTeamParams) error {
	// Get current user
	user := accessmodel.UserFromContext(ctx)

	// Delete team
	err := h.organizations.DeleteTeam(ctx, params.ID, user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to delete team")
	}

	return nil
}

// ListTeamMembers handles GET /teams/{id}/members
func (h *CoreServer) ListTeamMembers(ctx context.Context, params corev1.ListTeamMembersParams) (*corev1.ListTeamMembersResponseModel, error) {
	// Get current user
	user := accessmodel.UserFromContext(ctx)

	// Get members
	members, err := h.organizations.ListTeamMembers(ctx, params.ID, user.Id, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to list team members")
	}

	return projectResponse(h, ctx, listTeamMembersDTO(members.Items, members.Pagination))
}

// AddTeamMember handles POST /teams/{id}/members
func (h *CoreServer) AddTeamMember(ctx context.Context, params corev1.AddTeamMemberParams) error {
	user := accessmodel.UserFromContext(ctx)

	role := organizationsmodel.TeamRoleMember
	if params.Role.IsSet() && params.Role.Value != "" {
		role = organizationsmodel.TeamRole(params.Role.Value)
	}

	input := &organizationsmodel.AddTeamMemberInput{
		TeamID: params.ID,
		UserID: params.UserID,
		Role:   role,
	}

	err := h.organizations.AddTeamMember(ctx, input, user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to add team member")
	}

	return nil
}

// UpdateTeamMemberRole handles PATCH /teams/{id}/members
func (h *CoreServer) UpdateTeamMemberRole(ctx context.Context, params corev1.UpdateTeamMemberRoleParams) error {
	user := accessmodel.UserFromContext(ctx)

	err := h.organizations.UpdateTeamMemberRole(ctx, params.ID, params.UserID, organizationsmodel.TeamRole(params.Role), user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to update team member role")
	}

	return nil
}

// RemoveTeamMember handles DELETE /teams/{id}/members
func (h *CoreServer) RemoveTeamMember(ctx context.Context, params corev1.RemoveTeamMemberParams) error {
	user := accessmodel.UserFromContext(ctx)

	err := h.organizations.RemoveTeamMember(ctx, params.ID, params.UserID, user.Id)
	if err != nil {
		return wrapTeamUCError(err, "failed to remove team member")
	}

	return nil
}

// ListTeamContests handles GET /teams/{id}/contests
func (h *CoreServer) ListTeamContests(ctx context.Context, params corev1.ListTeamContestsParams) (*corev1.ListContestsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)

	contests, err := h.organizations.GetTeamContests(ctx, params.ID, user.Id, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to list team contests")
	}

	contestsList := &contestsmodel.ContestsList{Contests: contests.Items, Pagination: contests.Pagination}

	return projectResponse(h, ctx, ListContestsResponseDTO(contestsList))
}

// ListTeamProblems handles GET /teams/{id}/problems
func (h *CoreServer) ListTeamProblems(ctx context.Context, params corev1.ListTeamProblemsParams) (*corev1.ListProblemsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)

	problems, err := h.organizations.GetTeamProblems(ctx, params.ID, user.Id, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, wrapTeamUCError(err, "failed to list team problems")
	}

	problemsList := &problemsmodel.ProblemsList{Problems: problems.Items, Pagination: problems.Pagination}

	return projectResponse(h, ctx, ListProblemsResponseDTO(problemsList))
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
	// Adding context must not change the use case error classification.
	// Unclassified failures still map to HTTP 500 in pkg.ToREST.
	return pkg.Wrap(nil, err, fallbackMsg)
}
