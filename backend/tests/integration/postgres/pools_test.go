//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// queryRecorder records every statement a pool sends, including Exec and BEGIN,
// so tests can assert query counts without reaching into repository internals.
type queryRecorder struct {
	mu      sync.Mutex
	queries []string
}

func (r *queryRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queries = append(r.queries, data.SQL)
	return ctx
}

func (r *queryRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (r *queryRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queries = nil
}

func (r *queryRecorder) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.queries)
}

// count returns the number of recorded statements containing fragment.
func (r *queryRecorder) count(fragment string) int {
	n := 0
	for _, query := range r.recorded() {
		if strings.Contains(query, fragment) {
			n++
		}
	}
	return n
}

func newTestPool(t *testing.T, configure func(*pgxpool.Config)) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(testDSN(t))
	require.NoError(t, err)
	configure(config)
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// newRecordingPool returns a pool on the top-level test's database that records its statements.
func newRecordingPool(t *testing.T) (*pgxpool.Pool, *queryRecorder) {
	t.Helper()
	recorder := &queryRecorder{}
	pool := newTestPool(t, func(config *pgxpool.Config) { config.ConnConfig.Tracer = recorder })
	return pool, recorder
}

// blockerPool runs transactions that hold locks. Its unique application name lets
// tests find the sessions it blocks without knowing which locks are involved.
type blockerPool struct {
	*pgxpool.Pool
	name string
}

func newBlockerPool(t *testing.T) blockerPool {
	t.Helper()
	name := "gate-test-blocker-" + uuid.NewString()
	pool := newTestPool(t, func(config *pgxpool.Config) { config.ConnConfig.RuntimeParams["application_name"] = name })
	return blockerPool{Pool: pool, name: name}
}

// blocked returns the number of sessions waiting for a lock held by this pool.
func (b blockerPool) blocked(ctx context.Context) (int, error) {
	var n int
	err := b.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity waiter WHERE EXISTS (
		SELECT 1 FROM pg_stat_activity holder WHERE holder.application_name=$1 AND holder.pid=ANY(pg_blocking_pids(waiter.pid)))`, b.name).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count blocked sessions: %w", err)
	}
	return n, nil
}

func (b blockerPool) waitBlocked(t *testing.T, ctx context.Context, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		n, err := b.blocked(ctx)
		return err == nil && n >= count
	}, 3*time.Second, 5*time.Millisecond)
}

var errReleased = errors.New("held transaction rolled back")

// holdTransaction starts run, which must call barrier inside its transaction once
// the work whose locks are held is done. The transaction stays open until the
// returned release is called; release(true) commits and release(false) rolls back.
// Uncommitted transactions are rolled back at cleanup, which also cancels ctx so
// a run stuck before its barrier cannot keep a connection and hang pool.Close.
func holdTransaction(t *testing.T, run func(ctx context.Context, barrier func() error) error) (release func(commit bool) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready, commit, done := make(chan struct{}), make(chan bool, 1), make(chan error, 1)
	go func() {
		done <- run(ctx, func() error {
			close(ready)
			if <-commit {
				return nil
			}
			return errReleased
		})
	}()
	var once sync.Once
	var result error
	release = func(c bool) error {
		once.Do(func() {
			commit <- c
			result = <-done
			if !c && errors.Is(result, errReleased) {
				result = nil
			}
		})
		return result
	}
	t.Cleanup(func() {
		cancel()
		_ = release(false)
	})
	select {
	case <-ready:
	case err := <-done:
		done <- err // Keep the result for release.
		require.NoError(t, err)
		t.Fatal("held transaction finished without reaching its barrier")
	case <-time.After(5 * time.Second):
		t.Fatal("held transaction did not reach its barrier")
	}
	return release
}
