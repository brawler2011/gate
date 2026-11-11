//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Expensive services are started at most once per package run and shared by all tests;
// tests isolate their data instead (truncated tables, a bucket per test).
var (
	shared   sharedContainers
	sharedMu sync.Mutex
)

type sharedContainers struct {
	containers []testcontainers.Container
	s3Endpoint string
	judge      *goJudge
}

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

// newSeaweedBackedS3Storage returns the package's real S3-compatible store and a fresh,
// empty bucket owned by the calling test.
func newSeaweedBackedS3Storage(t *testing.T) (*storage.S3Storage, string) {
	t.Helper()
	ctx := context.Background()
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared.s3Endpoint == "" {
		c, err := startShared(ctx, "SeaweedFS", testcontainers.ContainerRequest{
			Image:        "chrislusf/seaweedfs:4.53_full",
			ExposedPorts: []string{seaweedS3Port},
			Cmd:          []string{"server", "-s3", "-filer", "-ip.bind=0.0.0.0", "-volume.max=100", "-master.volumeSizeLimitMB=32", "-dir=/data"},
			WaitingFor:   wait.ForHTTP("/").WithPort(seaweedS3Port).WithStartupTimeout(60 * time.Second),
		})
		require.NoError(t, err)
		host, err := c.Host(ctx)
		require.NoError(t, err)
		port, err := c.MappedPort(ctx, seaweedS3Port)
		require.NoError(t, err)
		shared.s3Endpoint = fmt.Sprintf("http://%s:%s", host, port.Port())
	}
	store := storage.NewS3Storage(storage.S3Config{Endpoint: shared.s3Endpoint, AccessKey: "any", SecretKey: "any", Region: "us-east-1"})
	bucket := "t-" + uuid.NewString()
	require.NoError(t, store.EnsureBucket(ctx, bucket))
	return store, bucket
}

// newGoJudge returns the package's real sandbox started from the project's image.
func newGoJudge(t *testing.T) (*sandbox.Client, *sandbox.FileTransport) {
	t.Helper()
	ctx := context.Background()
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared.judge == nil {
		request := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{Context: "../../../judge", Dockerfile: "Dockerfile", KeepImage: true},
			ExposedPorts:   []string{goJudgeHTTPPort, goJudgeGRPCPort}, Privileged: true,
			Cmd:        []string{"-enable-grpc", "-grpc-addr=0.0.0.0:5051", "-http-addr=0.0.0.0:5050", "-dir=/tmp/gate-files", "-parallelism=2"},
			WaitingFor: wait.ForHTTP("/version").WithPort(goJudgeHTTPPort).WithStartupTimeout(e2eSandboxTimout)}
		// CI prebuilds the image; locally it is built from the project's Dockerfile.
		if image := os.Getenv("GOJUDGE_TEST_IMAGE"); image != "" {
			request.FromDockerfile = testcontainers.FromDockerfile{}
			request.Image = image
		}
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
