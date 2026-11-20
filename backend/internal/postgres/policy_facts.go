package postgres

import (
	"context"
	"encoding/json"

	"github.com/brawler2011/gate/backend/internal/access"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PolicyFactsRepo uses pool reads only; it must never bind authorization locks.
type PolicyFactsRepo struct{ q *sqlc.Queries }

func NewPolicyFactsRepo(pool *pgxpool.Pool) *PolicyFactsRepo {
	return &PolicyFactsRepo{q: sqlc.New(pool)}
}

var _ access.PolicyFacts = (*PolicyFactsRepo)(nil)

func (r *PolicyFactsRepo) LoadProblemFacts(ctx context.Context, viewer uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]m.ProblemContext, error) {
	result := make(map[uuid.UUID]m.ProblemContext, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.q.LoadProblemFacts(ctx, sqlc.LoadProblemFactsParams{ViewerID: &viewer, Ids: ids})
	if err != nil {
		return nil, dbError("load problem facts", err)
	}
	for _, row := range rows {
		var facts m.ProblemContext
		if err := json.Unmarshal(row.Facts, &facts); err != nil {
			return nil, dbError("decode policy facts", err)
		}
		result[row.ID] = facts
	}
	return result, nil
}
func (r *PolicyFactsRepo) LoadContestFacts(ctx context.Context, viewer uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]m.ContestContext, error) {
	result := make(map[uuid.UUID]m.ContestContext, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.q.LoadContestFacts(ctx, sqlc.LoadContestFactsParams{ViewerID: &viewer, Ids: ids})
	if err != nil {
		return nil, dbError("load contest facts", err)
	}
	for _, row := range rows {
		var facts m.ContestContext
		if err := json.Unmarshal(row.Facts, &facts); err != nil {
			return nil, dbError("decode policy facts", err)
		}
		result[row.ID] = facts
	}
	return result, nil
}
func (r *PolicyFactsRepo) LoadTeamFacts(ctx context.Context, viewer uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]m.TeamContext, error) {
	result := make(map[uuid.UUID]m.TeamContext, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.q.LoadTeamFacts(ctx, sqlc.LoadTeamFactsParams{ViewerID: &viewer, Ids: ids})
	if err != nil {
		return nil, dbError("load team facts", err)
	}
	for _, row := range rows {
		var facts m.TeamContext
		if err := json.Unmarshal(row.Facts, &facts); err != nil {
			return nil, dbError("decode policy facts", err)
		}
		result[row.ID] = facts
	}
	return result, nil
}

func (r *PolicyFactsRepo) LoadOrganizationFacts(ctx context.Context, viewer uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]m.OrganizationContext, error) {
	result := make(map[uuid.UUID]m.OrganizationContext, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.q.LoadOrganizationFacts(ctx, sqlc.LoadOrganizationFactsParams{ViewerID: &viewer, Ids: ids})
	if err != nil {
		return nil, dbError("load organization facts", err)
	}
	for _, row := range rows {
		var facts m.OrganizationContext
		if err := json.Unmarshal(row.Facts, &facts); err != nil {
			return nil, dbError("decode policy facts", err)
		}
		result[row.ID] = facts
	}
	return result, nil
}

func (r *PolicyFactsRepo) LoadCapabilityUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]m.User, error) {
	result := make(map[uuid.UUID]m.User, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.q.LoadCapabilityUsers(ctx, ids)
	if err != nil {
		return nil, dbError("load capability users", err)
	}
	for _, row := range rows {
		var u m.User
		if err := json.Unmarshal(row.Facts, &u); err != nil {
			return nil, dbError("decode capability user", err)
		}
		result[row.ID] = u
	}
	return result, nil
}

// AccessRevision is a pool read. Failure invalidates WS leases immediately.
func (r *PolicyFactsRepo) AccessRevision(ctx context.Context) (int64, error) {
	return r.q.GetAccessRevision(ctx)
}
