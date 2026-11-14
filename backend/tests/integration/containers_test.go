//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/tests/judgeimage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Expensive services are started at most once per package run and shared by all tests;
// tests isolate their data instead (a database and a bucket per test), so they can run
// in parallel.
var (
	shared   sharedContainers
	sharedMu sync.Mutex
)

type sharedContainers struct {
	containers []testcontainers.Container
	postgres   string // connection string of the maintenance database; empty until started
	s3Endpoint string
	judge      *goJudge
}

const (
	postgresPort = "5432/tcp"
	// templateDatabase holds the migrated schema that every test database is cloned from.
	templateDatabase = "gate_template"
)

type goJudge struct {
	client *sandbox.Client
	files  *sandbox.FileTransport
}

func TestMain(m *testing.M) {
	code := m.Run()
	if shared.judge != nil {
		_ = shared.judge.client.Close()
	}
	for _, c := range shared.containers {
		if err := terminate(c); err != nil {
			log.Printf("terminate shared container: %v", err)
		}
	}
	os.Exit(code)
}

// terminate removes a throwaway container without a graceful stop; SeaweedFS ignores
// SIGTERM, so a graceful stop always waits for Docker's full stop timeout.
func terminate(c testcontainers.Container) error {
	return c.Terminate(context.Background(), testcontainers.StopTimeout(0))
}

func startShared(ctx context.Context, name string, request testcontainers.ContainerRequest) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	if c != nil {
		shared.containers = append(shared.containers, c)
	}
	if err != nil {
		return nil, fmt.Errorf("start shared %s: %w", name, err)
	}
	log.Printf("started shared %s container", name)
	return c, nil
}

// newTestDB returns a pool for a fresh, migrated database owned by the calling test,
// cloned from the template database of the package's PostgreSQL container.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	admin := sharedPostgres(t)
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	conn, err := pgx.Connect(ctx, admin)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateDatabase)
	require.NoError(t, errors.Join(err, conn.Close(ctx)))
	pool, err := pkg.NewPostgresDB(withDatabase(t, admin, name))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// sharedPostgres starts the package's PostgreSQL container on first use, migrates the
// template database and returns the maintenance database's connection string.
func sharedPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared.postgres != "" {
		return shared.postgres
	}
	c, err := startShared(ctx, "PostgreSQL", testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{postgresPort},
		Env:          map[string]string{"POSTGRES_PASSWORD": "postgres", "POSTGRES_DB": templateDatabase},
		// Every parallel test holds its own pool; durability is irrelevant for throwaway data.
		Cmd:        []string{"postgres", "-c", "max_connections=1000", "-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off"},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60 * time.Second),
	})
	require.NoError(t, err)
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, postgresPort)
	require.NoError(t, err)
	admin := fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port.Port())

	db, err := sql.Open("pgx", withDatabase(t, admin, templateDatabase))
	require.NoError(t, err)
	_, file, _, _ := runtime.Caller(0)
	require.NoError(t, goose.SetDialect("postgres"))
	err = goose.Up(db, filepath.Join(filepath.Dir(file), "../../migrations"))
	// A template database cannot be cloned while anyone is connected to it.
	require.NoError(t, errors.Join(err, db.Close()))
	shared.postgres = admin
	return admin
}

func withDatabase(t *testing.T, connString, database string) string {
	t.Helper()
	u, err := url.Parse(connString)
	require.NoError(t, err)
	u.Path = "/" + database
	return u.String()
}

// newSeaweedBackedS3Storage returns the package's real S3-compatible store and a fresh,
// empty bucket owned by the calling test.
func newSeaweedBackedS3Storage(t *testing.T) (*storage.S3Storage, string) {
	t.Helper()
	ctx := context.Background()
	endpoint := sharedSeaweedEndpoint(t)
	store := storage.NewS3Storage(storage.S3Config{Endpoint: endpoint, AccessKey: "any", SecretKey: "any", Region: "us-east-1"})
	bucket := "t-" + uuid.NewString()
	require.NoError(t, store.EnsureBucket(ctx, bucket))
	return store, bucket
}

// sharedSeaweedEndpoint holds the lock only for shared container initialization.
func sharedSeaweedEndpoint(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared.s3Endpoint == "" {
		c, err := startShared(ctx, "SeaweedFS", testcontainers.ContainerRequest{
			Image:        "chrislusf/seaweedfs:4.53_full",
			ExposedPorts: []string{seaweedS3Port},
			Cmd:          []string{"server", "-s3", "-filer", "-ip.bind=0.0.0.0", "-volume.max=1000", "-master.volumeSizeLimitMB=32", "-dir=/data"},
			WaitingFor:   wait.ForHTTP("/").WithPort(seaweedS3Port).WithStartupTimeout(60 * time.Second),
		})
		require.NoError(t, err)
		host, err := c.Host(ctx)
		require.NoError(t, err)
		port, err := c.MappedPort(ctx, seaweedS3Port)
		require.NoError(t, err)
		shared.s3Endpoint = fmt.Sprintf("http://%s:%s", host, port.Port())
	}
	return shared.s3Endpoint
}

// newGoJudge returns the package's real sandbox started from the project's image.
func newGoJudge(t *testing.T) (*sandbox.Client, *sandbox.FileTransport) {
	t.Helper()
	ctx := context.Background()
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared.judge == nil {
		image, err := judgeimage.Ref()
		require.NoError(t, err)
		request := testcontainers.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{goJudgeHTTPPort, goJudgeGRPCPort}, Privileged: true,
			Cmd:        []string{"-enable-grpc", "-grpc-addr=0.0.0.0:5051", "-http-addr=0.0.0.0:5050", "-dir=/tmp/gate-files", "-parallelism=2"},
			WaitingFor: wait.ForHTTP("/version").WithPort(goJudgeHTTPPort).WithStartupTimeout(e2eSandboxTimout)}
		c, err := startShared(ctx, "go-judge", request)
		require.NoError(t, err)
		host, err := c.Host(ctx)
		require.NoError(t, err)
		httpPort, err := c.MappedPort(ctx, goJudgeHTTPPort)
		require.NoError(t, err)
		grpcPort, err := c.MappedPort(ctx, goJudgeGRPCPort)
		require.NoError(t, err)
		client, err := sandbox.NewClient(host + ":" + grpcPort.Port())
		require.NoError(t, err)
		files, err := sandbox.NewFileTransport("http://"+host+":"+httpPort.Port(), &http.Client{Timeout: time.Minute})
		if err != nil {
			_ = client.Close()
		}
		require.NoError(t, err)
		shared.judge = &goJudge{client: client, files: files}
	}
	return shared.judge.client, shared.judge.files
}
