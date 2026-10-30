package pg

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WorkspaceLocker dedicates a session to each operation. Critical database writes
// use this same session, so a dead owner cannot commit through the repository pool.
type WorkspaceLocker struct{ config *pgx.ConnConfig }

func NewWorkspaceLocker(config *pgx.ConnConfig) *WorkspaceLocker {
	return &WorkspaceLocker{config: config.Copy()}
}

type workspaceSessionKey struct{}
type workspaceSession struct {
	mu     sync.Mutex
	conn   *pgx.Conn
	cancel context.CancelCauseFunc
}

func (s *workspaceSession) lost(err error) error {
	failure := fmt.Errorf("workspace lock session lost: %w", err)
	s.cancel(failure)
	return failure
}

func (s *workspaceSession) Check(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("workspace operation canceled: %w", err)
	}
	probe, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := s.conn.Ping(probe); err != nil {
		return s.lost(err)
	}
	return nil
}

// Serialize whole generated queries (including Scan) with heartbeat and release.
func withWorkspaceQueries(ctx context.Context, fallback *sqlc.Queries, fn func(context.Context, *sqlc.Queries) error) error {
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("workspace operation canceled: %w", err)
	}
	session, ok := ctx.Value(workspaceSessionKey{}).(*workspaceSession)
	if !ok {
		return fn(ctx, fallback)
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("workspace operation canceled: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := fn(queryCtx, sqlc.New(session.conn))
	if session.conn.IsClosed() {
		return errors.Join(err, session.lost(errors.New("owning PostgreSQL connection is closed")))
	}
	return err
}

func (l *WorkspaceLocker) WithWorkspaceLock(ctx context.Context, id uuid.UUID, fn func(context.Context) error) (result error) {
	conn, err := pgx.ConnectConfig(ctx, l.config)
	if err != nil {
		return fmt.Errorf("connect workspace lock: %w", err)
	}
	closeSession := func() error {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := conn.Close(closeCtx); err != nil {
			return fmt.Errorf("close workspace lock session: %w", err)
		}
		return nil
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", "gate-workspace:"+id.String()); err != nil {
		return errors.Join(fmt.Errorf("acquire workspace lock: %w", err), closeSession())
	}
	operation, cancel := context.WithCancelCause(ctx)
	session := &workspaceSession{conn: conn, cancel: cancel}
	operation = context.WithValue(operation, workspaceSessionKey{}, session)
	operation = interfaces.WithWorkspaceOwnership(operation, session)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-operation.Done():
				return
			case <-ticker.C:
				if session.Check(operation) != nil {
					return
				}
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
		session.mu.Lock()
		result = errors.Join(result, context.Cause(operation), closeSession())
		session.mu.Unlock()
		cancel(context.Canceled)
	}()
	result = fn(operation)
	return errors.Join(result, session.Check(operation))
}
