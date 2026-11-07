package postgres

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type authorization struct{ permissions *access.PermissionsUseCase }

func bindAuthorization(q *sqlc.Queries) *authorization {
	return &authorization{permissions: access.NewPermissionsUseCase(
		authorizationContests{&ContestsRepo{queries: q}},
		authorizationUsers{q},
		authorizationProblems{&ProblemsRepo{queries: q}},
		authorizationTeams{&TeamsRepo{q: q}},
		authorizationOrganizations{&OrganizationsRepo{q: q}},
	)}
}

func (a *authorization) HasContestPermission(ctx context.Context, contest, user uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	return a.permissions.HasContestPermission(ctx, contest, user, action)
}
func (a *authorization) HasOrganizationPermission(ctx context.Context, org, user uuid.UUID, action accessmodel.OrgAction) (bool, error) {
	return a.permissions.HasOrganizationPermission(ctx, org, user, action)
}
func (a *authorization) HasProblemPermission(ctx context.Context, problem, user uuid.UUID, action accessmodel.ProblemAction) (bool, error) {
	return a.permissions.HasProblemPermission(ctx, problem, user, action)
}

// Authorization reads hold mutable account privileges until the command commits.
type authorizationUsers struct{ queries *sqlc.Queries }

func (r authorizationUsers) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	user, err := r.queries.GetAuthorizationUser(ctx, id)
	if err != nil {
		return accessmodel.User{}, HandlePgErr(err)
	}
	return accessmodel.User{Id: user.ID, Username: user.Username, Role: user.Role, ExpiresAt: user.ExpiresAt}, nil
}

// Transactional authorization holds only the membership rows used for access.
// Ordinary repositories use nonlocking queries for pool-backed reads.
type authorizationOrganizations struct{ *OrganizationsRepo }

func (r authorizationOrganizations) GetMember(ctx context.Context, org, user uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	row, err := r.q.GetAuthorizationOrganizationMember(ctx, sqlc.GetAuthorizationOrganizationMemberParams{OrganizationID: org, UserID: user})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	return mapOrganizationMember(row), nil
}

type authorizationContests struct{ *ContestsRepo }

func (r authorizationContests) GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	row, err := r.queries.GetAuthorizationContestMember(ctx, sqlc.GetAuthorizationContestMemberParams{ContestID: c.ContestId, UserID: c.UserId})
	if err != nil {
		return contestsmodel.ContestMember{}, HandlePgErr(err)
	}
	return mapContestMember(sqlc.GetContestMemberRow(row)), nil
}

type authorizationProblems struct{ *ProblemsRepo }

func (r authorizationProblems) GetProblemMember(ctx context.Context, problem, user uuid.UUID) (problemsmodel.ProblemMember, error) {
	row, err := r.queries.GetAuthorizationProblemMember(ctx, sqlc.GetAuthorizationProblemMemberParams{ProblemID: problem, UserID: user})
	if err != nil {
		return problemsmodel.ProblemMember{}, HandlePgErr(err)
	}
	return mapProblemMember(row), nil
}

type authorizationTeams struct{ *TeamsRepo }

func (r authorizationTeams) GetUserTeamsByOrganization(ctx context.Context, user, org uuid.UUID) ([]organizationsmodel.Team, error) {
	rows, err := r.q.GetAuthorizationUserTeamsByOrganization(ctx, sqlc.GetAuthorizationUserTeamsByOrganizationParams{UserID: user, OrganizationID: org})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	return mapTeams(rows), nil
}

func (r authorizationTeams) GetTeamMember(ctx context.Context, team, user uuid.UUID) (*organizationsmodel.TeamMember, error) {
	row, err := r.q.GetAuthorizationTeamMember(ctx, sqlc.GetAuthorizationTeamMemberParams{TeamID: team, UserID: user})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	return &organizationsmodel.TeamMember{TeamID: row.TeamID, UserID: row.UserID, Role: organizationsmodel.TeamRole(row.Role), CreatedAt: row.CreatedAt}, nil
}

// Organization creation needs to keep the creator's privileges stable without
// serializing unrelated creations by the same administrator.
type organizationUsers struct{ *UsersRepo }

func (r organizationUsers) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	return (authorizationUsers{r.queries}).GetUserById(ctx, id)
}
