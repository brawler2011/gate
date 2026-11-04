package problems

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func withinProblem(ctx context.Context, transactions ProblemTransactions, id uuid.UUID, action accessmodel.ProblemAction, fn func(ProblemTxStore) error) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	return transactions.Within(ctx, func(tx ProblemTxStore) error {
		problem, err := tx.GetProblemById(ctx, id)
		if err != nil {
			return err
		}
		if err := tx.LockOrganization(ctx, problem.OrganizationID); err != nil {
			return err
		}
		if err := tx.LockProblem(ctx, id); err != nil {
			return err
		}
		allowed, err := tx.HasProblemPermission(ctx, id, user.Id, action)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}
		return fn(tx)
	})
}
