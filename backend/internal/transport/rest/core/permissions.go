package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/access"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

type PermissionLoader interface {
	Load(context.Context, access.PermissionIDs) (*access.PermissionSet, error)
}

// projectResponse fills the permissions of every resource in response; parent names the contest of contest-scoped responses.
func projectResponse[T any](h *CoreServer, ctx context.Context, response *T, parent ...uuid.UUID) (*T, error) {
	if h.Permissions == nil {
		return response, nil
	}
	var ids access.PermissionIDs
	fills := permissionFills(response, &ids, parent...)
	set, err := h.Permissions.Load(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, fill := range fills {
		if err := fill(set); err != nil {
			return nil, err
		}
	}
	return response, nil
}

type permissionFill func(*access.PermissionSet) error

func addFill[P, M any](fills *[]permissionFill, dst *M, convert func(P) M, get func(*access.PermissionSet) (P, error)) {
	*fills = append(*fills, func(s *access.PermissionSet) error {
		p, err := get(s)
		*dst = convert(p)
		return err
	})
}

func permissionFills(response any, ids *access.PermissionIDs, parent ...uuid.UUID) []permissionFill {
	var fills []permissionFill
	var parentID uuid.UUID
	if len(parent) > 0 {
		parentID = parent[0]
	}
	org := func(id uuid.UUID, dst *corev1.OrganizationPermissionsModel) {
		ids.Organizations = append(ids.Organizations, id)
		addFill(&fills, dst, func(p access.OrganizationPermissions) corev1.OrganizationPermissionsModel {
			return corev1.OrganizationPermissionsModel(p)
		},
			func(s *access.PermissionSet) (access.OrganizationPermissions, error) { return s.Organization(id) })
	}
	team := func(id uuid.UUID, dst *corev1.TeamPermissionsModel) {
		ids.Teams = append(ids.Teams, id)
		addFill(&fills, dst, func(p access.TeamPermissions) corev1.TeamPermissionsModel { return corev1.TeamPermissionsModel(p) },
			func(s *access.PermissionSet) (access.TeamPermissions, error) { return s.Team(id) })
	}
	problem := func(id uuid.UUID, dst *corev1.ProblemPermissionsModel) {
		ids.Problems = append(ids.Problems, id)
		addFill(&fills, dst, func(p access.ProblemPermissions) corev1.ProblemPermissionsModel {
			return corev1.ProblemPermissionsModel(p)
		},
			func(s *access.PermissionSet) (access.ProblemPermissions, error) { return s.Problem(id) })
	}
	contest := func(id uuid.UUID, dst *corev1.ContestPermissionsModel) {
		ids.Contests = append(ids.Contests, id)
		addFill(&fills, dst, func(p access.ContestPermissions) corev1.ContestPermissionsModel {
			return corev1.ContestPermissionsModel(p)
		},
			func(s *access.PermissionSet) (access.ContestPermissions, error) { return s.Contest(id) })
	}
	contestProblem := func(id, problemID uuid.UUID, dst *corev1.ContestProblemPermissionsModel) {
		ids.Contests = append(ids.Contests, id)
		ids.Problems = append(ids.Problems, problemID)
		addFill(&fills, dst, func(p access.ContestProblemPermissions) corev1.ContestProblemPermissionsModel {
			return corev1.ContestProblemPermissionsModel(p)
		}, func(s *access.PermissionSet) (access.ContestProblemPermissions, error) {
			return s.ContestProblem(id, problemID)
		})
	}
	member := func(scope *[]uuid.UUID, id uuid.UUID, dst *corev1.MemberPermissionsModel, get func(*access.PermissionSet) (access.MemberPermissions, error)) {
		*scope = append(*scope, id)
		addFill(&fills, dst, func(p access.MemberPermissions) corev1.MemberPermissionsModel {
			return corev1.MemberPermissionsModel(p)
		}, get)
	}
	switch r := response.(type) {
	case *corev1.ListOrganizationsResponseModel:
		for i := range r.Organizations {
			org(r.Organizations[i].ID, &r.Organizations[i].Permissions)
		}
	case *corev1.GetOrganizationResponseModel:
		org(r.Organization.ID, &r.Organization.Permissions)
	case *corev1.ListTeamsResponseModel:
		for i := range r.Teams {
			team(r.Teams[i].ID, &r.Teams[i].Permissions)
		}
	case *corev1.GetTeamResponseModel:
		team(r.Team.ID, &r.Team.Permissions)
	case *corev1.ListProblemsResponseModel:
		for i := range r.Problems {
			problem(r.Problems[i].ID, &r.Problems[i].Permissions)
		}
	case *corev1.GetProblemResponseModel:
		problem(r.Problem.ID, &r.Problem.Permissions)
	case *corev1.ListContestsResponseModel:
		for i := range r.Contests {
			contest(r.Contests[i].ID, &r.Contests[i].Permissions)
		}
	case *corev1.ListUserContestsResponseModel:
		for i := range r.Contests {
			contest(r.Contests[i].ID, &r.Contests[i].Permissions)
		}
	case *corev1.GetContestResponseModel:
		contest(r.Contest.ID, &r.Contest.Permissions)
		for i := range r.Problems {
			contestProblem(r.Contest.ID, r.Problems[i].ProblemID, &r.Problems[i].Permissions)
		}
	case *corev1.GetContestProblemResponseModel:
		contestProblem(parentID, r.Problem.ProblemID, &r.Problem.Permissions)
	case *corev1.GetUserDashboardResponseModel:
		for i := range r.RecentContests {
			contest(r.RecentContests[i].ID, &r.RecentContests[i].Permissions)
		}
		for i := range r.MyProblems {
			problem(r.MyProblems[i].ID, &r.MyProblems[i].Permissions)
		}
	case *corev1.ListOrganizationMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			member(&ids.Organizations, v.OrganizationID, &v.Permissions, func(s *access.PermissionSet) (access.MemberPermissions, error) {
				return s.OrganizationMember(v.OrganizationID, v.UserID, m.OrganizationRole(v.Role))
			})
		}
	case *corev1.ListTeamMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			member(&ids.Teams, v.TeamID, &v.Permissions, func(s *access.PermissionSet) (access.MemberPermissions, error) {
				return s.TeamMember(v.TeamID, v.UserID, m.TeamRole(v.Role))
			})
		}
	case *corev1.ListProblemMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			member(&ids.Problems, v.ProblemID, &v.Permissions, func(s *access.PermissionSet) (access.MemberPermissions, error) {
				return s.ProblemMember(v.ProblemID, v.UserID, m.ProblemRole(v.Role))
			})
		}
	case *corev1.ListContestMembersResponseModel:
		for i := range r.Members {
			v := &r.Members[i]
			member(&ids.Contests, v.ContestID, &v.Permissions, func(s *access.PermissionSet) (access.MemberPermissions, error) {
				return s.ContestMember(v.ContestID, v.UserID, m.ContestRole(v.ContestRole))
			})
		}
	case *corev1.ListProblemTeamsResponseModel:
		for i := range r.Teams {
			v := &r.Teams[i]
			member(&ids.Problems, v.ProblemID, &v.Permissions, func(s *access.PermissionSet) (access.MemberPermissions, error) {
				return s.ProblemTeam(v.ProblemID, m.ProblemRole(v.Role))
			})
		}
	case *corev1.ListContestTeamsResponseModel:
		for i := range r.Teams {
			v := &r.Teams[i]
			member(&ids.Contests, v.ContestID, &v.Permissions, func(s *access.PermissionSet) (access.MemberPermissions, error) {
				return s.ContestTeam(v.ContestID, m.ContestRole(v.ContestRole))
			})
		}
	}
	return fills
}
