//go:build integration

package pg

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestWorkspaceLocker_PostgresSessions(t *testing.T) {
	ctx := context.Background()
	config := workspaceLockTestConfig(t)
	first, second := NewWorkspaceLocker(config), NewWorkspaceLocker(config)
	id := uuid.New()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- first.WithWorkspaceLock(ctx, id, func(context.Context) error { close(entered); <-release; return nil })
	}()
	<-entered
	// Another session cannot enter the same workspace; cancellation interrupts its wait.
	canceled, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	called := false
	err := second.WithWorkspaceLock(canceled, id, func(context.Context) error { called = true; return nil })
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, called)
	// Independent problems remain usable while this lock is held.
	require.NoError(t, second.WithWorkspaceLock(ctx, uuid.New(), func(context.Context) error { return nil }))
	close(release)
	require.NoError(t, <-done)
	// Successful and failed callbacks both close the owning session and release its lock.
	fault := errors.New("storage failed")
	require.ErrorIs(t, second.WithWorkspaceLock(ctx, id, func(context.Context) error { return fault }), fault)
	retryCtx, retryCancel := context.WithTimeout(ctx, 5*time.Second)
	defer retryCancel()
	require.NoError(t, first.WithWorkspaceLock(retryCtx, id, func(context.Context) error { return nil }))
}

func workspaceLockTestConfig(t *testing.T) *pgx.ConnConfig {
	t.Helper()
	if dsn := os.Getenv("WORKSPACE_LOCK_TEST_DSN"); dsn != "" {
		config, err := pgx.ParseConfig(dsn)
		require.NoError(t, err)
		return config
	}
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:17-alpine", postgres.WithDatabase("workspace"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(ctx)) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	config, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	return config
}
