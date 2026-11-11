//go:build integration

package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/problems/builder"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// pipelineServices are the real S3-compatible store and sandbox the pipeline runs against.
type pipelineServices struct {
	store  *storage.S3Storage
	client *sandbox.Client
	files  *sandbox.FileTransport
	langs  map[string]sandbox.Language
}

func startPipelineServices(t *testing.T) pipelineServices {
	t.Helper()
	ctx := context.Background()
	s3container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: "chrislusf/seaweedfs:4.53_full", ExposedPorts: []string{"8333/tcp"},
		Cmd:        []string{"server", "-s3", "-filer", "-ip.bind=0.0.0.0", "-volume.max=100", "-master.volumeSizeLimitMB=1024", "-dir=/data"},
		WaitingFor: wait.ForHTTP("/").WithPort("8333/tcp").WithStartupTimeout(time.Minute)}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s3container.Terminate(context.Background())) })
	s3host, err := s3container.Host(ctx)
	require.NoError(t, err)
	s3port, err := s3container.MappedPort(ctx, "8333/tcp")
	require.NoError(t, err)
	store := storage.NewS3Storage(storage.S3Config{Endpoint: fmt.Sprintf("http://%s:%s", s3host, s3port.Port()), AccessKey: "test", SecretKey: "test", Region: "us-east-1"})
	require.NoError(t, store.EnsureBucket(ctx, "objects"))
	request := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{Context: "../../../judge", Dockerfile: "Dockerfile", KeepImage: true},
		ExposedPorts:   []string{"5050/tcp", "5051/tcp"}, Privileged: true,
		Cmd:        []string{"-enable-grpc", "-grpc-addr=0.0.0.0:5051", "-http-addr=0.0.0.0:5050", "-dir=/tmp/gate-files", "-parallelism=1"},
		WaitingFor: wait.ForHTTP("/version").WithPort("5050/tcp")}
	// CI prebuilds the image; locally it is built from the project's Dockerfile.
	if image := os.Getenv("GOJUDGE_TEST_IMAGE"); image != "" {
		request.FromDockerfile = testcontainers.FromDockerfile{}
		request.Image = image
	}
	judge, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, judge.Terminate(context.Background())) })
	host, err := judge.Host(ctx)
	require.NoError(t, err)
	hp, err := judge.MappedPort(ctx, "5050/tcp")
	require.NoError(t, err)
	gp, err := judge.MappedPort(ctx, "5051/tcp")
	require.NoError(t, err)
	client, err := sandbox.NewClient(host + ":" + gp.Port())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	files, err := sandbox.NewFileTransport("http://"+host+":"+hp.Port(), &http.Client{Timeout: time.Minute})
	require.NoError(t, err)
	cfg, err := sandbox.LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	return pipelineServices{store: store, client: client, files: files, langs: cfg.Langs}
}

func TestProblemPipelineRealServices(t *testing.T) {
	ctx, pool, repo, org, d := draftFixture(t)
	services := startPipelineServices(t)
	store, client, files := services.store, services.client, services.files
	cfg := struct{ Langs map[string]sandbox.Language }{services.langs}
	checker, generator, solution, validator, subtask := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	d.Files = map[string]string{
		"checkers/check.cpp":     "#include <fstream>\nint main(int n,char**v){long long a,b;std::ifstream out(v[2]),ans(v[3]);return !(out>>a) || !(ans>>b) || a!=b;}",
		"generators/gen.py":      "import sys\nprint(sys.argv[1], sys.argv[2])\n",
		"solutions/main.cpp":     "#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<'\\n';}",
		"validators/validate.py": "import sys\na,b=map(int,sys.stdin.read().split())\nassert a>=0 and b>=0\n",
		"statements/en.md":       "A snapshot statement.",
	}
	d.Components = []model.DraftComponent{{ID: checker, Role: "checker", Path: "checkers/check.cpp", Language: "cpp"}, {ID: generator, Role: "generator", Path: "generators/gen.py", Language: "python"}, {ID: solution, Role: "solution", Path: "solutions/main.cpp", Language: "cpp"}, {ID: validator, Role: "validator", Path: "validators/validate.py", Language: "python"}}
	d.CheckerID = &checker
	d.ValidatorID = &validator
	d.MainSolutionID = &solution
	d.Subtasks = []model.DraftSubtask{{ID: subtask, Ordinal: 1, Name: "All", Policy: "complete"}}
	seed := int64(7)
	d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: subtask, Sample: true, Input: model.DraftInput{Mode: "generator", ComponentID: &generator, Args: []string{"10"}, Seed: &seed}, Answer: model.DraftAnswer{Mode: "generated", ComponentID: &solution}}}
	_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	d, err = repo.GetDraft(ctx, d.ProblemID)
	require.NoError(t, err)
	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 10 * time.Second, Memory: 256 << 20, Processes: 50, Output: 32 << 20, Diagnostic: 4096}
	compile := aux
	compile.CPU = 10 * time.Second
	compile.Wall = 20 * time.Second
	compile.Memory = 512 << 20
	compile.Processes = 150
	environment := model.Toolchain{Digest: "sha256:" + strings.Repeat("a", 64), Architecture: "amd64", ABI: "linux-gnu", Languages: cfg.Langs}
	limits := model.BuildLimits{DiskBytes: 256 << 20, FileCount: 100, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: compile, Auxiliary: aux}
	_, err = repo.EnqueueBuild(ctx, mutationKey(org, "build", d.Revision), d.ProblemID, d.Revision, ProblemBuildOptions{Toolchain: environment, Limits: limits, MaxAttempts: 3, Duration: time.Minute, Lease: time.Minute})
	require.NoError(t, err)
	d.Title = "Changed after enqueue"
	d.Files["solutions/main.cpp"] = "invalid later source"
	_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	b, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, b.ID)
	temp := dbfixtures.DiskTempDir(t)
	pipeline := builder.Pipeline{Repo: repo, Store: store, Bucket: "objects", TempDir: temp, Client: client, Files: files, Environment: environment, UploadTTL: time.Minute, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	pid, err := pipeline.Execute(ctx, b)
	require.NoError(t, err)
	published, err := repo.GetBuild(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", published.State)
	var title, key, digest string
	var size int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT p.title,o.key,o.sha256,o.size FROM problem_packages p JOIN stored_objects o ON o.id=p.object_id WHERE p.id=$1`, pid).Scan(&title, &key, &digest, &size))
	require.Equal(t, "Initial", title)
	requireStoredObject(t, ctx, store, "objects", key, size, digest)
	// The second finalization resolves the original success, never a second Package.
	again, err := repo.PublishBuild(ctx, b.ID, *b.LeaseToken, uuid.New())
	require.NoError(t, err)
	require.Equal(t, pid, again)
	var outputs int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM problem_build_objects WHERE build_id=$1 AND role='output'`, b.ID).Scan(&outputs))
	require.Zero(t, outputs)
}

func requireStoredObject(t *testing.T, ctx context.Context, store storage.Storage, bucket, key string, size int64, digest string) {
	t.Helper()
	r, _, err := store.DownloadFile(ctx, bucket, key, nil)
	require.NoError(t, err)
	defer r.Close()
	h := sha256.New()
	n, err := io.Copy(h, r)
	require.NoError(t, err)
	require.Equal(t, size, n)
	require.Equal(t, digest, hex.EncodeToString(h.Sum(nil)))
}
