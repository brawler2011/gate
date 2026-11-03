package postgres

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
)

type authorization struct{ permissions *access.PermissionsUseCase }

func bindAuthorization(q *sqlc.Queries) *authorization {
	return &authorization{permissions: access.NewPermissionsUseCase(&ContestsRepo{queries: q}, authorizationUsers{q}, &ProblemsRepo{queries: q}, &TeamsRepo{q: q}, &OrganizationsRepo{q: q})}
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
