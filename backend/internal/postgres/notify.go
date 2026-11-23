package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Notifier struct{ queries *sqlc.Queries }

func NewNotifier(pool *pgxpool.Pool) *Notifier { return &Notifier{queries: sqlc.New(pool)} }

func (n *Notifier) Notify(ctx context.Context, notice eventdata.Notice) error {
	payload, err := json.Marshal(notice)
	if err != nil {
		return fmt.Errorf("encode notice %s: %w", notice.Type, err)
	}
	if err := n.queries.NotifyEvents(ctx, string(payload)); err != nil {
		return dbError("notify", err)
	}
	return nil
}

// Channel names a LISTEN query; channel names live only in notify.sql.
type Channel struct {
	name   string
	listen func(*sqlc.Queries, context.Context) error
}

var (
	EventsChannel    = Channel{"events", (*sqlc.Queries).ListenEvents}
	JudgeJobsChannel = Channel{"judge jobs", (*sqlc.Queries).ListenJudgeJobs}
)

// Listen calls handle for each notification on channel until ctx ends, reconnecting on failure.
// connected runs once LISTEN is active after every (re)connect: notifications sent while disconnected are lost.
func Listen(ctx context.Context, pool *pgxpool.Pool, channel Channel, connected func(), handle func(*pgconn.Notification)) error {
	for {
		err := listenOnce(ctx, pool, channel, connected, handle)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.WarnContext(ctx, "postgres listener disconnected", "channel", channel.name, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func listenOnce(ctx context.Context, pool *pgxpool.Pool, channel Channel, connected func(), handle func(*pgconn.Notification)) error {
	pooled, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listener connection: %w", err)
	}
	// A listening connection must never return to the pool.
	conn := pooled.Hijack()
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if err := channel.listen(sqlc.New(conn), ctx); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	connected()
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait for notification: %w", err)
		}
		handle(notification)
	}
}
