package core

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
)

func (h *CoreServer) ListTeams(ctx context.Context, params corev1.ListTeamsParams) (*corev1.ListTeamsResponseModel, error) {
	filter := &organizationsmodel.TeamsFilter{
		Search:   params.Search.Or(""),
		Page:     params.Page.Or(1),
		PageSize: params.PageSize.Or(50),
	}
	if params.OrganizationID.IsSet() {
		filter.OrganizationID = &params.OrganizationID.Value
	}
	result, err := h.Teams.ListTeams(ctx, filter)
	if err != nil {
		return nil, err
	}
	return projectResponse(h, ctx, listTeamsDTO(result.Teams, result.Pagination))
}

func (h *CoreServer) CreateTeam(ctx context.Context, req *corev1.CreateTeamReq) (*corev1.CreationResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "missing request body")
	}

	if err := validateCreateTeamRequest(req.Name, req.OrganizationID); err != nil {
		return nil, err
	}

	slug := generateLogin(req.Name)

	input := &organizationsmodel.CreateTeamInput{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Slug:           slug,
		Description:    "",
		Privacy:        organizationsmodel.TeamPrivacyClosed,
		ParentTeamID:   nil,
	}

	team, err := h.Teams.CreateTeam(ctx, input)
	if err != nil {
		return nil, err
	}

	return &corev1.CreationResponseModel{
		ID: team.ID,
	}, nil
}

func (h *CoreServer) GetTeam(ctx context.Context, params corev1.GetTeamParams) (*corev1.GetTeamResponseModel, error) {
	team, err := h.Teams.GetTeam(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, &corev1.GetTeamResponseModel{
		Team: teamDTO(*team),
	})
}

func (h *CoreServer) UpdateTeam(ctx context.Context, req *corev1.UpdateTeamRequestModel, params corev1.UpdateTeamParams) error {
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

	input := &organizationsmodel.UpdateTeamInput{
		Name:        reqName,
		Description: reqDesc,
		Privacy:     nil, // Privacy is not exposed by this endpoint.
	}

	err := h.Teams.UpdateTeam(ctx, params.ID, input)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteTeam(ctx context.Context, params corev1.DeleteTeamParams) error {
	err := h.Teams.DeleteTeam(ctx, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListTeamMembers(ctx context.Context, params corev1.ListTeamMembersParams) (*corev1.ListTeamMembersResponseModel, error) {
	members, err := h.Teams.ListTeamMembers(ctx, params.ID, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, listTeamMembersDTO(members.Items, members.Pagination))
}

func (h *CoreServer) AddTeamMember(ctx context.Context, params corev1.AddTeamMemberParams) error {
	role := accessmodel.TeamMember
	if params.Role.IsSet() && params.Role.Value != "" {
		role = accessmodel.TeamRole(params.Role.Value)
	}

	input := &organizationsmodel.AddTeamMemberInput{
		TeamID: params.ID,
		UserID: params.UserID,
		Role:   role,
	}

	err := h.Teams.AddTeamMember(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateTeamMemberRole(ctx context.Context, params corev1.UpdateTeamMemberRoleParams) error {
	err := h.Teams.UpdateTeamMemberRole(ctx, params.ID, params.UserID, accessmodel.TeamRole(params.Role))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) RemoveTeamMember(ctx context.Context, params corev1.RemoveTeamMemberParams) error {
	err := h.Teams.RemoveTeamMember(ctx, params.ID, params.UserID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListTeamContests(ctx context.Context, params corev1.ListTeamContestsParams) (*corev1.ListContestsResponseModel, error) {
	contests, err := h.Teams.GetTeamContests(ctx, params.ID, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, err
	}

	contestsList := &contestsmodel.ContestsList{Contests: contests.Items, Pagination: contests.Pagination}

	return projectResponse(h, ctx, ListContestsResponseDTO(contestsList))
}

func (h *CoreServer) ListTeamProblems(ctx context.Context, params corev1.ListTeamProblemsParams) (*corev1.ListProblemsResponseModel, error) {
	problems, err := h.Teams.GetTeamProblems(ctx, params.ID, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, err
	}

	problemsList := &problemsmodel.ProblemsList{Problems: problems.Items, Pagination: problems.Pagination}

	return projectResponse(h, ctx, ListProblemsResponseDTO(problemsList))
}
