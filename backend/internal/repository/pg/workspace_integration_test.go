//go:build integration

package pg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestWorkspacePackageReferencePostgres(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("WORKSPACE_TEST_DSN")
	if dsn == "" {
		container, err := postgres.Run(ctx, "postgres:18-alpine", postgres.WithDatabase("gate"), postgres.WithUsername("gate"), postgres.WithPassword("gate"), testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, container.Terminate(ctx)) })
		dsn, err = container.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
	}
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, admin.Close(ctx)) })
	schema := "workspace_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); require.NoError(t, err) })
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `CREATE TABLE problems (id uuid PRIMARY KEY, title text NOT NULL, time_limit_ms integer NOT NULL, memory_limit_mb integer NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(), visibility text, owner_id uuid, is_template boolean, manifest jsonb)`)
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "20261030000000_file_workspace_packages.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, strings.Split(string(migration), "-- +goose Down")[0])
	require.NoError(t, err)
	id := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO problems(id,title,time_limit_ms,memory_limit_mb) VALUES ($1,'Initial',1000,256)`, id)
	require.NoError(t, err)
	repo := NewProblemsRepo(pool)
	hash, err := repo.GetWorkspaceHash(ctx, id)
	require.NoError(t, err)
	require.Empty(t, hash)
	require.NoError(t, repo.CommitWorkspace(ctx, id, "", "first", &models.WorkspaceMetadata{Title: "First", TimeLimitMs: 1234, MemoryLimitMb: 128}))
	// Real concurrent writers from the same reference: exactly one wins.
	results := make(chan error, 2)
	for _, next := range []string{"second-a", "second-b"} {
		go func() {
			results <- repo.CommitWorkspace(ctx, id, "first", next, &models.WorkspaceMetadata{Title: next, TimeLimitMs: 2345, MemoryLimitMb: 512})
		}()
	}
	a, b := <-results, <-results
	require.True(t, a == nil || b == nil)
	if a == nil {
		require.ErrorIs(t, b, pkg.ErrConflict)
	} else {
		require.ErrorIs(t, a, pkg.ErrConflict)
		require.NoError(t, b)
	}
	var title string
	var timeMs, memoryMb int
	err = pool.QueryRow(ctx, `SELECT workspace_hash,title,time_limit_ms,memory_limit_mb FROM problems WHERE id=$1`, id).Scan(&hash, &title, &timeMs, &memoryMb)
	require.NoError(t, err)
	require.Equal(t, hash, title)
	require.Equal(t, 2345, timeMs)
	require.Equal(t, 512, memoryMb)
	require.ErrorIs(t, repo.CommitWorkspace(ctx, id, "first", "stale", &models.WorkspaceMetadata{Title: "Stale"}), pkg.ErrConflict)
	current, err := repo.GetWorkspaceHash(ctx, id)
	require.NoError(t, err)
	require.Equal(t, hash, current)
	// The catalog endpoint cannot independently overwrite an initialized title.
	changed := "Independent title"
	require.NoError(t, repo.UpdateProblem(ctx, id, &models.ProblemUpdate{Title: &changed}))
	err = pool.QueryRow(ctx, `SELECT title FROM problems WHERE id=$1`, id).Scan(&title)
	require.NoError(t, err)
	require.Equal(t, hash, title)
}
