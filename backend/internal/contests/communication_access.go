package contests

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func withinContestEvents(ctx context.Context, transactions ContestEventsTransactions, id uuid.UUID, action accessmodel.ContestAction, fn func(ContestEventsTxStore) error) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	return transactions.Within(ctx, func(tx ContestEventsTxStore) error {
		allowed, err := tx.HasContestPermission(ctx, id, user.Id, action)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}
		return fn(tx)
	})
}
