package contests

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

func withinContestEvents(ctx context.Context, transactions ContestEventsTransactions, id uuid.UUID, action accessmodel.ContestAction, fn func(ContestEventsTxStore, access.ContestAccess) error) error {
	return transactions.Within(ctx, func(tx ContestEventsTxStore) error {
		a, err := access.Contest(ctx, tx, id, action)
		if err != nil {
			return err
		}
		return fn(tx, a)
	})
}
