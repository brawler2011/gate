package postgres

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func readSnapshot[T any](ctx context.Context, pool *pgxpool.Pool, read func(*sqlc.Queries) (T, error)) (T, error) {
	var zero T
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return zero, HandlePgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := read(sqlc.New(tx))
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, HandlePgErr(err)
	}
	return result, nil
}
