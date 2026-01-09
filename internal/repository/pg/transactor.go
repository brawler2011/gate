package pg

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Transactor struct {
	db *pgxpool.Pool
}

func NewTransactor(db *pgxpool.Pool) *Transactor {
	return &Transactor{
		db: db,
	}
}

func (m *Transactor) WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return HandlePgErr(err)
	}

	if err := fn(ctx, tx); err != nil {
		errRollback := tx.Rollback(ctx)
		return HandlePgErr(errors.Join(err, errRollback))
	}

	if err := tx.Commit(ctx); err != nil {
		return HandlePgErr(err)
	}

	return nil
}
