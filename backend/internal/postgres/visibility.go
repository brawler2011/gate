package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
)

// SQL emits exactly one null resource when the page is empty, including when
// offset exceeds the total. Removing this sentinel is not access filtering.
func emptyScopeRow(data []byte) bool { return len(data) == 0 || string(data) == "null" }

func scopeProblems(ctx context.Context, q *sqlc.Queries, params sqlc.ScopeProblemsParams) ([]problemsmodel.Problem, int32, error) {
	rows, err := q.ScopeProblems(ctx, params)
	if err != nil {
		return nil, 0, HandlePgErr(err)
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
			return nil, 0, fmt.Errorf("decode scoped resource: %w", err)
		}
		items = append(items, mapGetProblemByIDRow(resource))
	}
	return items, total, nil
}
func scopeContests(ctx context.Context, q *sqlc.Queries, params sqlc.ScopeContestsParams) ([]contestsmodel.Contest, int32, error) {
	rows, err := q.ScopeContests(ctx, params)
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}
	items := make([]contestsmodel.Contest, 0, len(rows))
	var total int32
	for _, row := range rows {
		total = safeInt32(row.TotalItems)
		if emptyScopeRow(row.Resource) {
			continue
		}
		var resource struct {
			sqlc.GetContestByIDRow
			Settings json.RawMessage `json:"settings"`
		}
		if err := json.Unmarshal(row.Resource, &resource); err != nil {
			return nil, 0, fmt.Errorf("decode scoped contest: %w", err)
		}
		resource.GetContestByIDRow.Settings = resource.Settings
		items = append(items, mapGetContestByIDRow(resource.GetContestByIDRow))
	}
	return items, total, nil
}

func readViewerID(ctx context.Context) *uuid.UUID {
	id := accessmodel.UserFromContext(ctx).Id
	return &id
}
