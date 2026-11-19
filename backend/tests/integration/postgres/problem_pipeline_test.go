//go:build integration

package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/brawler2011/gate/backend/tests/judgeimage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

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
	t.Cleanup(func() { require.NoError(t, s3container.Terminate(context.Background(), testcontainers.StopTimeout(0))) })
	s3host, err := s3container.Host(ctx)
	require.NoError(t, err)
	s3port, err := s3container.MappedPort(ctx, "8333/tcp")
	require.NoError(t, err)
	store := storage.NewS3Storage(storage.S3Config{Endpoint: fmt.Sprintf("http://%s:%s", s3host, s3port.Port()), AccessKey: "test", SecretKey: "test", Region: "us-east-1"})
	require.NoError(t, store.EnsureBucket(ctx, "objects"))
	image, err := judgeimage.Ref()
	require.NoError(t, err)
	request := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"5050/tcp"}, Privileged: true,
		Cmd:        []string{"-http-addr=0.0.0.0:5050", "-dir=/tmp/gate-files", "-parallelism=1", "-no-fallback"},
		WaitingFor: wait.ForHTTP("/version").WithPort("5050/tcp")}
	judge, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, judge.Terminate(context.Background(), testcontainers.StopTimeout(0))) })
	host, err := judge.Host(ctx)
	require.NoError(t, err)
	hp, err := judge.MappedPort(ctx, "5050/tcp")
	require.NoError(t, err)
	client, err := sandbox.NewClient("http://"+host+":"+hp.Port(), &http.Client{})
	require.NoError(t, err)
	files, err := sandbox.NewFileTransport("http://"+host+":"+hp.Port(), &http.Client{Timeout: time.Minute})
	require.NoError(t, err)
	cfg, err := sandbox.LoadConfig("../../../../judge/langs.toml")
	require.NoError(t, err)
	return pipelineServices{store: store, client: client, files: files, langs: cfg.Langs}
}

func TestProblemPipelineRealServices(t *testing.T) {
	t.Parallel()
	ctx, pool, repo, org, d := draftFixture(t)
	services := startPipelineServices(t)
	store, client, files := services.store, services.client, services.files
	cfg := &sandbox.Config{Langs: services.langs}
	generator, solution, validator, subtask := "gen", "main", "validate", uuid.New()
	d.Checker = checker("cpp17", "#include <fstream>\nint main(int n,char**v){long long a,b;std::ifstream out(v[2]),ans(v[3]);return !(out>>a) || !(ans>>b) || a!=b;}")
	d.Generators = []model.DraftNamedProgram{named(generator, "python3", "import sys\nprint(sys.argv[1], sys.argv[2])\n")}
	d.Solutions = []model.DraftNamedProgram{named(solution, "cpp17", "#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<'\\n';}")}
	d.Validators = []model.DraftValidator{{DraftNamedProgram: named(validator, "python3", "import sys\na,b=map(int,sys.stdin.read().split())\nassert a>=0 and b>=0\n")}}
	d.Statements = []model.DraftStatement{{Language: "en", Legend: "A snapshot statement."}}
	d.MainSolution = &solution
	d.Subtasks = []model.DraftSubtask{{ID: subtask, Ordinal: 1, Name: "All", Policy: "complete"}}
	d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: subtask, Sample: true, Input: model.DraftInput{Mode: "generator", Generator: &generator, Args: []string{"10", "7"}}, Answer: model.DraftAnswer{Mode: "generated", Solution: &solution}}}
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
	limits := model.BuildLimits{DiskBytes: 256 << 20, FileCount: 100, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: compile, Auxiliary: aux}
	_, err = repo.EnqueueBuild(ctx, mutationKey(org, "build", d.Revision), d.ProblemID, d.Revision, postgres.ProblemBuildOptions{Languages: cfg.Langs, Limits: limits, MaxAttempts: 3, Duration: time.Minute, Lease: time.Minute})
	require.NoError(t, err)
	d.Title = "Changed after enqueue"
	d.Solutions[0].Source = "invalid later source"
	_, err = repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
	require.NoError(t, err)
	b, err := repo.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, b.ID)
	temp := dbfixtures.DiskTempDir(t)
	pipeline := builder.Pipeline{Repo: repo, Store: store, Bucket: "objects", TempDir: temp, Client: client, Files: files, Languages: cfg, Programs: sandbox.NewProgramCache(dbfixtures.DiskCache(t), client, files, sandbox.Image{Config: cfg}), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
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
	again, err := repo.PublishBuild(ctx, b.ID, *b.LeaseToken, model.NewStoredObject(b.OrganizationID, model.UploadSpec{Purpose: "package", SHA256: digest, Size: size}), nil)
	require.NoError(t, err)
	require.Equal(t, pid, again)
	require.Zero(t, countRows(t, ctx, pool, `SELECT count(*) FROM stored_objects o WHERE o.purpose='package' AND NOT EXISTS(SELECT 1 FROM problem_packages p WHERE p.object_id=o.id)`), "the retry records no second object")
}


// Every validator that applies to a test checks it; a group validator sees only its group, with --group.
func TestProblemPipelineRunsEveryValidator(t *testing.T) {
	t.Parallel()
	ctx, _, repo, org, d := draftFixture(t)
	services := startPipelineServices(t)
	cfg := &sandbox.Config{Langs: services.langs}
	generator, solution, small, large := "gen", "main", uuid.New(), uuid.New()
	d.Checker = checker("python3", "pass\n")
	d.Generators = []model.DraftNamedProgram{named(generator, "python3", "import sys\nprint(sys.argv[1], sys.argv[2])\n")}
	d.Solutions = []model.DraftNamedProgram{named(solution, "python3", "print(sum(map(int, input().split())))\n")}
	d.Validators = []model.DraftValidator{
		{DraftNamedProgram: named("nonneg", "python3", "a, b = map(int, input().split())\nassert a >= 0 and b >= 0\n")},
		{DraftNamedProgram: named("small", "python3", "import sys\nassert sys.argv[1:] == ['--group', 'small'], sys.argv\nassert max(map(int, input().split())) < 10\n"), Subtasks: []uuid.UUID{small}},
	}
	d.MainSolution = &solution
	d.Subtasks = []model.DraftSubtask{{ID: small, Ordinal: 1, Name: "small", Policy: "complete"}, {ID: large, Ordinal: 2, Name: "large", Policy: "complete"}}
	test := func(ordinal int, subtask uuid.UUID, a, b string) model.DraftTest {
		return model.DraftTest{ID: uuid.New(), Ordinal: ordinal, SubtaskID: subtask, Input: model.DraftInput{Mode: "generator", Generator: &generator, Args: []string{a, b}}, Answer: model.DraftAnswer{Mode: "generated", Solution: &solution}}
	}
	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 10 * time.Second, Memory: 256 << 20, Processes: 50, Output: 32 << 20, Diagnostic: 4096}
	limits := model.BuildLimits{DiskBytes: 256 << 20, FileCount: 100, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: aux, Auxiliary: aux}
	pipeline := builder.Pipeline{Repo: repo, Store: services.store, Bucket: "objects", TempDir: dbfixtures.DiskTempDir(t), Client: services.client, Files: services.files, Languages: cfg,
		Programs: sandbox.NewProgramCache(dbfixtures.DiskCache(t), services.client, services.files, sandbox.Image{Config: cfg}), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	build := func(tests ...model.DraftTest) error {
		d.Tests = tests
		_, err := repo.SaveDraft(ctx, mutationKey(org, "edit", d), d)
		require.NoError(t, err)
		d, err = repo.GetDraft(ctx, d.ProblemID)
		require.NoError(t, err)
		_, err = repo.EnqueueBuild(ctx, mutationKey(org, "build", d.Revision), d.ProblemID, d.Revision, postgres.ProblemBuildOptions{Languages: cfg.Langs, Limits: limits, MaxAttempts: 3, Duration: time.Minute, Lease: time.Minute})
		require.NoError(t, err)
		b, err := repo.ClaimBuild(ctx, time.Minute)
		require.NoError(t, err)
		_, err = pipeline.Execute(ctx, b)
		return err
	}

	// The small validator would reject 50 50, so it must not run on the large group.
	require.NoError(t, build(test(1, small, "1", "2"), test(2, large, "50", "50")))
	for _, tc := range []struct{ a, validator string }{{"-1", "nonneg"}, {"12", "small"}} {
		var failure *builder.Failure
		require.ErrorAs(t, build(test(1, large, "50", "50"), test(2, small, tc.a, "1")), &failure)
		require.Equal(t, "validate", failure.Stage)
		require.Equal(t, 2, failure.Test)
		require.Contains(t, failure.Message, "validator "+tc.validator+":")
	}
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
