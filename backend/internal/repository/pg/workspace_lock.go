package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WorkspaceLocker uses a dedicated connection: storage calls must not exhaust the
// repository pool while holding a lock. Closing the session always releases it.
type WorkspaceLocker struct{ config *pgx.ConnConfig }

func NewWorkspaceLocker(config *pgx.ConnConfig) *WorkspaceLocker {
	return &WorkspaceLocker{config: config.Copy()}
}

func (l *WorkspaceLocker) WithWorkspaceLock(ctx context.Context, id uuid.UUID, fn func(context.Context) error) error {
	conn, err := pgx.ConnectConfig(ctx, l.config)
	if err != nil {
		return fmt.Errorf("connect workspace lock: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", "gate-workspace:"+id.String()); err != nil {
		return fmt.Errorf("acquire workspace lock: %w", err)
	}
	return fn(ctx)
}
