//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/brawler2011/gate/backend/internal/datareset"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDataResetOnSchemaChange checks that the same schema keeps every store's data and
// a changed schema wipes it all. A reset empties every bucket, so the test has its own
// SeaweedFS rather than the shared one.
func TestDataResetOnSchemaChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stores := newResetStores(t, ctx)

	_, file, _, _ := runtime.Caller(0)
	migrations := os.DirFS(filepath.Join(filepath.Dir(file), "../../migrations"))

	reset, err := datareset.Apply(ctx, stores, migrations)
	require.NoError(t, err)
	require.True(t, reset, "an empty database has no schema hash")

	// Data in every store.
	_, err = stores.DB.ExecContext(ctx, "CREATE TABLE reset_marker (x int)")
	require.NoError(t, err)
	_, err = stores.DB.ExecContext(ctx, "INSERT INTO reset_marker VALUES (1)")
	require.NoError(t, err)
	require.NoError(t, stores.Redis.Set(ctx, "reset-marker", "1", 0).Err())
	require.NoError(t, stores.S3.EnsureBucket(ctx, "reset-marker"))
	require.NoError(t, stores.S3.UploadFile(ctx, "reset-marker", "object", strings.NewReader("data"), "text/plain"))
	_, err = stores.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: "RESET_MARKER", Subjects: []string{"reset.>"}})
	require.NoError(t, err)

	reset, err = datareset.Apply(ctx, stores, migrations)
	require.NoError(t, err)
	require.False(t, reset)
	var rows int
	require.NoError(t, stores.DB.QueryRowContext(ctx, "SELECT count(*) FROM reset_marker").Scan(&rows))
	require.Equal(t, 1, rows)
	require.Equal(t, "1", stores.Redis.Get(ctx, "reset-marker").Val())
	keys, err := stores.S3.ListFiles(ctx, "reset-marker", "")
	require.NoError(t, err)
	require.Equal(t, []string{"object"}, keys)
	_, err = stores.JetStream.Stream(ctx, "RESET_MARKER")
	require.NoError(t, err)

	changed := changedMigrations(t, migrations)
	reset, err = datareset.Apply(ctx, stores, changed)
	require.NoError(t, err)
	require.True(t, reset)

	var marker sql.NullString
	require.NoError(t, stores.DB.QueryRowContext(ctx, "SELECT to_regclass('public.reset_marker')::text").Scan(&marker))
	require.False(t, marker.Valid, "the reset drops every table")
	var users sql.NullString
	require.NoError(t, stores.DB.QueryRowContext(ctx, "SELECT to_regclass('public.users')::text").Scan(&users))
	require.True(t, users.Valid, "the reset applies the schema")
	var hash string
	require.NoError(t, stores.DB.QueryRowContext(ctx, "SELECT hash FROM schema_hash").Scan(&hash))
	want, err := datareset.Hash(changed)
	require.NoError(t, err)
	require.Equal(t, want, hash)
	require.Zero(t, stores.Redis.Exists(ctx, "reset-marker").Val())
	keys, err = stores.S3.ListFiles(ctx, "reset-marker", "")
	require.NoError(t, err)
	require.Empty(t, keys)
	_, err = stores.JetStream.Stream(ctx, "RESET_MARKER")
	require.ErrorIs(t, err, jetstream.ErrStreamNotFound)
}

// changedMigrations returns a copy of migrations with a comment appended to each file.
func changedMigrations(t *testing.T, migrations fs.FS) fs.FS {
	t.Helper()
	names, err := fs.Glob(migrations, "*.sql")
	require.NoError(t, err)
	changed := fstest.MapFS{}
	for _, name := range names {
		data, err := fs.ReadFile(migrations, name)
		require.NoError(t, err)
		changed[name] = &fstest.MapFile{Data: append(data, "\n-- changed\n"...)}
	}
	return changed
}

func newResetStores(t *testing.T, ctx context.Context) datareset.Stores {
	t.Helper()

	admin := sharedPostgres(t)
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	conn, err := pgx.Connect(ctx, admin)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	require.NoError(t, conn.Close(ctx))
	db, err := sql.Open("pgx", withDatabase(t, admin, name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	redisAddr := startResetContainer(t, ctx, testcontainers.ContainerRequest{
		Image: "redis:7.2.4-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp"),
	}, "6379/tcp")
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = redisClient.Close() })

	js, _ := newNATSJetStream(t)

	s3Addr := startResetContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        "chrislusf/seaweedfs:4.53_full",
		ExposedPorts: []string{seaweedS3Port},
		Cmd:          []string{"server", "-s3", "-filer", "-ip.bind=0.0.0.0", "-volume.max=10", "-master.volumeSizeLimitMB=32", "-dir=/data"},
		WaitingFor:   wait.ForHTTP("/").WithPort(seaweedS3Port).WithStartupTimeout(60 * time.Second),
	}, seaweedS3Port)
	store := storage.NewS3Storage(storage.S3Config{Endpoint: "http://" + s3Addr, AccessKey: "any", SecretKey: "any", Region: "us-east-1"})

	return datareset.Stores{DB: db, Redis: redisClient, S3: store, JetStream: js}
}

// startResetContainer starts a container owned by the calling test and returns its
// address for port.
func startResetContainer(t *testing.T, ctx context.Context, request testcontainers.ContainerRequest, port nat.Port) string {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	if c != nil {
		t.Cleanup(func() { _ = terminate(c) })
	}
	require.NoError(t, err)
	host, err := c.Host(ctx)
	require.NoError(t, err)
	mapped, err := c.MappedPort(ctx, port)
	require.NoError(t, err)
	return fmt.Sprintf("%s:%s", host, mapped.Port())
}
