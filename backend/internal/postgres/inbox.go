package postgres

import (
	"context"
	"errors"
	"time"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Inbox struct{ pool *pgxpool.Pool }

func NewInbox(pool *pgxpool.Pool) *Inbox { return &Inbox{pool: pool} }

func (i *Inbox) Within(ctx context.Context, consumer string, eventID uuid.UUID, fn func(eventdata.Writer) error) error {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return HandlePgErr(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := sqlc.New(tx)
	_, err = q.RecordInboxReceipt(ctx, sqlc.RecordInboxReceiptParams{ConsumerName: consumer, EventID: eventID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return HandlePgErr(err)
	}
	if err := fn(&OutboxRepo{queries: q}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (i *Inbox) PruneReceipts(ctx context.Context, cutoff time.Time) (int64, error) {
	q := sqlc.New(i.pool)
	rows, err := q.PruneInboxReceipts(ctx, cutoff)
	if err != nil {
		return 0, HandlePgErr(err)
	}
	return rows, nil
}

