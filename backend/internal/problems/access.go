package problems

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

func withinProblem(ctx context.Context, transactions ProblemTransactions, id uuid.UUID, action accessmodel.ProblemAction, fn func(ProblemTxStore) error) error {
	return transactions.Within(ctx, func(tx ProblemTxStore) error {
		if err := access.Problem(ctx, tx, id, action); err != nil {
			return err
		}
		return fn(tx)
	})
}
