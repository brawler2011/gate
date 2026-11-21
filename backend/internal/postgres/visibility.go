package postgres

import (
	"context"
	"encoding/json"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
)

// Discard the null empty-page sentinel; visibility was already filtered by SQL.
func emptyScopeRow(data []byte) bool { return len(data) == 0 || string(data) == "null" }

func scopeProblems(ctx context.Context, q *sqlc.Queries, params sqlc.ScopeProblemsParams) ([]problemsmodel.Problem, int32, error) {
	rows, err := q.ScopeProblems(ctx, params)
	if err != nil {
		return nil, 0, dbError("scope problems", err)
	}
	items := make([]problemsmodel.Problem, 0, len(rows))
	var total int32
	for _, row := range rows {
		total = safeInt32(row.TotalItems)
		if emptyScopeRow(row.Resource) {
			continue
		}
		var resource sqlc.GetProblemByIDRow
		if err := json.Unmarshal(row.Resource, &resource); err != nil {
			return nil, 0, dbError("decode scoped resource", err)
		}
		items = append(items, mapGetProblemByIDRow(resource))
	}
	return items, total, nil
}
func scopeContests(ctx context.Context, q *sqlc.Queries, params sqlc.ScopeContestsParams) ([]contestsmodel.Contest, int32, error) {
	rows, err := q.ScopeContests(ctx, params)
	if err != nil {
		return nil, 0, dbError("scope contests", err)
	}
	items := make([]contestsmodel.Contest, 0, len(rows))
	var total int32
	for _, row := range rows {
		total = safeInt32(row.TotalItems)
		if emptyScopeRow(row.Resource) {
			continue
		}
		var resource sqlc.GetContestByIDRow
		if err := json.Unmarshal(row.Resource, &resource); err != nil {
			return nil, 0, dbError("decode scoped contest", err)
		}
		items = append(items, mapGetContestByIDRow(resource))
	}
	return items, total, nil
}

func readViewerID(ctx context.Context) *uuid.UUID {
	id := accessmodel.UserFromContext(ctx).Id
	return &id
}
