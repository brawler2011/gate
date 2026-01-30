package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/pkg"
)

// ListTeams handles GET /teams
func (h *CoreServer) ListTeams(ctx context.Context, request corev1.ListTeamsRequestObject) (corev1.ListTeamsResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "ListTeams not implemented yet")
}

// CreateTeam handles POST /teams
func (h *CoreServer) CreateTeam(ctx context.Context, request corev1.CreateTeamRequestObject) (corev1.CreateTeamResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "CreateTeam not implemented yet")
}

// DeleteTeam handles DELETE /teams/{id}
func (h *CoreServer) DeleteTeam(ctx context.Context, request corev1.DeleteTeamRequestObject) (corev1.DeleteTeamResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "DeleteTeam not implemented yet")
}

// GetTeam handles GET /teams/{id}
func (h *CoreServer) GetTeam(ctx context.Context, request corev1.GetTeamRequestObject) (corev1.GetTeamResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "GetTeam not implemented yet")
}

// UpdateTeam handles PATCH /teams/{id}
func (h *CoreServer) UpdateTeam(ctx context.Context, request corev1.UpdateTeamRequestObject) (corev1.UpdateTeamResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "UpdateTeam not implemented yet")
}

// RemoveTeamMember handles DELETE /teams/{id}/members
func (h *CoreServer) RemoveTeamMember(ctx context.Context, request corev1.RemoveTeamMemberRequestObject) (corev1.RemoveTeamMemberResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "RemoveTeamMember not implemented yet")
}

// ListTeamMembers handles GET /teams/{id}/members
func (h *CoreServer) ListTeamMembers(ctx context.Context, request corev1.ListTeamMembersRequestObject) (corev1.ListTeamMembersResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "ListTeamMembers not implemented yet")
}

// AddTeamMember handles POST /teams/{id}/members
func (h *CoreServer) AddTeamMember(ctx context.Context, request corev1.AddTeamMemberRequestObject) (corev1.AddTeamMemberResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "AddTeamMember not implemented yet")
}
