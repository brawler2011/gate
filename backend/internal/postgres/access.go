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
	return &authorization{permissions: access.NewPermissionsUseCase(&ContestsRepo{queries: q}, &UsersRepo{queries: q}, &ProblemsRepo{queries: q}, &TeamsRepo{q: q}, &OrganizationsRepo{q: q})}
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
