package postgres

import (
	"context"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/jackc/pgx/v5/pgxpool"
)

type guardedTransactions[T any] struct{ transactions[T] }

func (p guardedTransactions[T]) Prepare(ctx context.Context) (context.Context, error) {
	if _, ok := m.DecisionFromContext(ctx); ok {
		return ctx, nil
	}
	revision, err := sqlc.New(p.pool).GetAccessRevision(ctx)
	if err != nil {
		return ctx, HandlePgErr(err)
	}
	actor := m.UserFromContext(ctx)
	if actor.IsGuest() {
		return ctx, pkg.ErrUnauthenticated
	}
	expiry, err := sqlc.New(p.pool).GetCommandActorExpiry(ctx, actor.Id)
	if err != nil {
		return ctx, HandlePgErr(err)
	}
	if actor.ExpiresAt != nil && (expiry == nil || actor.ExpiresAt.Before(*expiry)) {
		expiry = actor.ExpiresAt
	}
	ctx = m.WithReadTime(ctx, m.ReadTime(ctx))
	return m.WithCommandDecision(ctx, m.CommandDecision{Revision: revision, ValidUntil: expiry}), nil
}

func newGuardedTransactions[T any](pool *pgxpool.Pool, bind transactions[T]) guardedTransactions[T] {
	bind.pool = pool
	return guardedTransactions[T]{bind}
}
