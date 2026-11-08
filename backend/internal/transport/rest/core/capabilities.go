package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/google/uuid"
)

type CapabilityProjector interface {
	Project(context.Context, []access.CapabilityRequest) ([][]string, error)
}
type capabilitySlot struct {
	request access.CapabilityRequest
	value   *[]string
}

// projectResponse binds typed DTO fields to one batch policy projection. No
// transport rule infers privileges from a role, creator ID or visibility.
func projectResponse[T any](h *CoreServer, ctx context.Context, response *T, parent ...uuid.UUID) (*T, error) {
	slots := capabilitySlots(response, parent...)
	requests := make([]access.CapabilityRequest, len(slots))
	for i, s := range slots {
		requests[i] = s.request
		*s.value = []string{}
	}
	if h.capabilities == nil {
		return response, nil
	} // Standalone transport fixtures.
	values, err := h.capabilities.Project(ctx, requests)
	if err != nil {
		return nil, err
	}
	for i, s := range slots {
		*s.value = values[i]
	}
	return response, nil
}
func capabilitySlots(response any, parent ...uuid.UUID) []capabilitySlot {
	slots := []capabilitySlot{}
	var parentID uuid.UUID
	if len(parent) > 0 {
		parentID = parent[0]
	}
	add := func(scope string, id uuid.UUID, value *[]string) {
		slots = append(slots, capabilitySlot{request: access.CapabilityRequest{Scope: scope, ID: id}, value: value})
	}
	row := func(scope, kind string, id, target uuid.UUID, role string, value *[]string) {
		slots = append(slots, capabilitySlot{request: access.CapabilityRequest{Scope: scope, ID: id, Kind: kind, Target: target, Role: role}, value: value})
	}
	switch r := response.(type) {
	case *corev1.ListOrganizationsResponseModel:
		for i := range r.Organizations {
			v := &r.Organizations[i]
			add("organization", v.ID, &v.Capabilities)
		}
	case *corev1.GetOrganizationResponseModel:
		add("organization", r.Organization.ID, &r.Organization.Capabilities)
	case *corev1.ListTeamsResponseModel:
		for i := range r.Teams {
			v := &r.Teams[i]
			add("team", v.ID, &v.Capabilities)
		}
	case *corev1.GetTeamResponseModel:
		add("team", r.Team.ID, &r.Team.Capabilities)
	case *corev1.ListProblemsResponseModel:
		for i := range r.Problems {
			v := &r.Problems[i]
			add("problem", v.ID, &v.Capabilities)
		}
	case *corev1.GetProblemResponseModel:
		add("problem", r.Problem.ID, &r.Problem.Capabilities)
	case *corev1.ListContestsResponseModel:
		for i := range r.Contests {
			v := &r.Contests[i]
			add("contest", v.ID, &v.Capabilities)
		}
	case *corev1.ListUserContestsResponseModel:
		for i := range r.Contests {
			v := &r.Contests[i]
			add("contest", v.ID, &v.Capabilities)
		}
	case *corev1.GetContestResponseModel:
		add("contest", r.Contest.ID, &r.Contest.Capabilities)
		for i := range r.Problems {
			v := &r.Problems[i]
			row("contest", "problem", r.Contest.ID, v.ProblemID, "", &v.Capabilities)
		}
	case *corev1.GetContestProblemResponseModel:
		row("contest", "problem", parentID, r.Problem.ProblemID, "", &r.Problem.Capabilities)
	case *corev1.GetMyContestRoleResponseModel:
		add("contest", parentID, &r.Capabilities)
	case *corev1.GetUserDashboardResponseModel:
		for i := range r.RecentContests {
			v := &r.RecentContests[i]
			add("contest", v.ID, &v.Capabilities)
		}
		for i := range r.MyProblems {
			v := &r.MyProblems[i]
			add("problem", v.ID, &v.Capabilities)
		}
	case *corev1.ListOrganizationMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			row("organization", "member", v.OrganizationID, v.UserID, string(v.Role), &v.Capabilities)
		}
	case *corev1.ListTeamMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			row("team", "member", v.TeamID, v.UserID, string(v.Role), &v.Capabilities)
		}
	case *corev1.ListProblemMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			row("problem", "member", v.ProblemID, v.UserID, string(v.Role), &v.Capabilities)
		}
	case *corev1.ListContestMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			row("contest", "member", v.ContestID, v.UserID, string(v.ContestRole), &v.Capabilities)
		}
	case *corev1.ListProblemTeamsResponseModel:
		for i := range r.Teams {
			v := &r.Teams[i]
			row("problem", "link", v.ProblemID, v.TeamID, string(v.Permission), &v.Capabilities)
		}
	case *corev1.ListContestTeamsResponseModel:
		for i := range r.Teams {
			v := &r.Teams[i]
			row("contest", "link", v.ContestID, v.TeamID, string(v.ContestRole), &v.Capabilities)
		}
	}
	return slots
}
