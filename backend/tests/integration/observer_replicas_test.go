//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type listener struct {
	connected chan struct{}
	notices   chan eventdata.Notice
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
		panic("unreachable")
	}
}

// Every replica must receive each committed notice, and must resync after losing its connection.
func TestNoticesReachEveryReplica(t *testing.T) {
	t.Parallel()
	pool := newTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	var running sync.WaitGroup
	t.Cleanup(func() { cancel(); running.Wait() })
	var replicas [2]listener
	for i := range replicas {
		r := listener{connected: make(chan struct{}, 2), notices: make(chan eventdata.Notice, 10)}
		replicas[i] = r
		running.Go(func() {
			_ = pgstore.Listen(ctx, pool, pgstore.EventsChannel, func() { r.connected <- struct{}{} }, func(n *pgconn.Notification) {
				var notice eventdata.Notice
				require.NoError(t, json.Unmarshal([]byte(n.Payload), &notice))
				r.notices <- notice
			})
		})
		receive(t, r.connected)
	}

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT pg_notify('gate_events', '{\"type\":\"rolled back\"}')")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	notice := eventdata.Notice{Type: "submissions.completed", ID: uuid.New()}
	require.NoError(t, pgstore.NewNotifier(pool).Notify(ctx, notice))
	for _, r := range replicas {
		require.Equal(t, notice, receive(t, r.notices))
	}

	_, err = pool.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE query LIKE '%LISTEN gate_events%' AND pid <> pg_backend_pid() AND datname = current_database()")
	require.NoError(t, err)
	for _, r := range replicas {
		receive(t, r.connected)
	}
}
