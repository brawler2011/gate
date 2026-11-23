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
	"github.com/jackc/pgx/v5/pgxpool"
)

// Access loads authorization facts through whatever connection its queries are bound to.
// Inside a command, lock keeps the actor's account row stable until the command commits.
type Access struct {
	q    *sqlc.Queries
	lock bool
}

var _ access.Loader = Access{}

func NewAccess(pool *pgxpool.Pool) Access { return Access{q: sqlc.New(pool)} }

func commandAccess(q *sqlc.Queries) Access { return Access{q: q, lock: true} }

func (a Access) Actor(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	if a.lock {
		row, err := a.q.LockAccessActor(ctx, id)
		if err != nil {
			return accessmodel.User{}, dbError("lock access actor", err)
		}
		return accessmodel.User{Id: row.ID, Role: row.Role, ExpiresAt: row.ExpiresAt}, nil
	}
	row, err := a.q.GetAccessActor(ctx, id)
	if err != nil {
		return accessmodel.User{}, dbError("get access actor", err)
	}
	return accessmodel.User{Id: row.ID, Role: row.Role, ExpiresAt: row.ExpiresAt}, nil
}

func (a Access) OrganizationFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.OrganizationFacts, error) {
	rows, err := a.q.LoadOrganizationAccess(ctx, sqlc.LoadOrganizationAccessParams{UserID: actor, Ids: ids})
	if err != nil {
		return nil, dbError("load organization access", err)
	}
	facts := make(map[uuid.UUID]access.OrganizationFacts, len(rows))
	for _, r := range rows {
		facts[r.ID] = access.OrganizationFacts{ID: r.ID, Role: accessmodel.OrganizationRole(r.Role), JoinPolicy: r.JoinPolicy}
	}
	return facts, nil
}

func (a Access) TeamFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.TeamFacts, error) {
	rows, err := a.q.LoadTeamAccess(ctx, sqlc.LoadTeamAccessParams{UserID: actor, Ids: ids})
	if err != nil {
		return nil, dbError("load team access", err)
	}
	facts := make(map[uuid.UUID]access.TeamFacts, len(rows))
	for _, r := range rows {
		facts[r.ID] = access.TeamFacts{ID: r.ID, OrganizationID: r.OrganizationID, Role: accessmodel.TeamRole(r.Role), Member: r.Member,
			Secret: organizationsmodel.TeamPrivacy(r.Privacy) == organizationsmodel.TeamPrivacySecret}
	}
	return facts, nil
}

func (a Access) ProblemFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.ProblemFacts, error) {
	rows, err := a.q.LoadProblemAccess(ctx, sqlc.LoadProblemAccessParams{UserID: actor, Ids: ids})
	if err != nil {
		return nil, dbError("load problem access", err)
	}
	facts := make(map[uuid.UUID]access.ProblemFacts, len(rows))
	for _, r := range rows {
		facts[r.ID] = access.ProblemFacts{ID: r.ID, OrganizationID: r.OrganizationID, Role: accessmodel.ProblemRole(r.Role),
			Public: r.Visibility == problemsmodel.ProblemVisibilityPublic}
	}
	return facts, nil
}

func (a Access) ContestFacts(ctx context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.ContestFacts, error) {
	rows, err := a.q.LoadContestAccess(ctx, sqlc.LoadContestAccessParams{UserID: actor, Ids: ids})
	if err != nil {
		return nil, dbError("load contest access", err)
	}
	facts := make(map[uuid.UUID]access.ContestFacts, len(rows))
	for _, r := range rows {
		f := access.ContestFacts{Contest: mapContest(r.Contest, r.OrgLogin), Role: accessmodel.ContestRole(r.Role), Now: r.Now, Blocked: r.Blocked, Competed: r.Competed}
		if r.RunStartedAt != nil && r.RunEndsAt != nil {
			f.Run = &contestsmodel.VirtualRun{UserID: actor, StartedAt: *r.RunStartedAt, EndsAt: *r.RunEndsAt}
		}
		facts[r.Contest.ID] = f
	}
	return facts, nil
}

// visibility returns the list parameters for the caller's grants.
func (a Access) visibility(ctx context.Context) (uuid.UUID, bool, error) {
	return access.Visibility(ctx, a)
}
