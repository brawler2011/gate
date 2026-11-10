//go:build integration
// +build integration

package integration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	judgeworker "github.com/brawler2011/gate/backend/internal/messaging/judge"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	seaweedS3Port    = "8333/tcp"
	objectsBucket    = "problem-objects"
	toolchainDigest  = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	goJudgeHTTPPort  = "5050/tcp"
	goJudgeGRPCPort  = "5051/tcp"
	e2eSandboxTimout = 90 * time.Second
)

// newSeaweedBackedS3Storage starts a real S3-compatible store with the immutable-object bucket.
func newSeaweedBackedS3Storage(t *testing.T) *storage.S3Storage {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "chrislusf/seaweedfs:4.53_full",
			ExposedPorts: []string{seaweedS3Port},
			Cmd:          []string{"server", "-s3", "-filer", "-ip.bind=0.0.0.0", "-volume.max=100", "-master.volumeSizeLimitMB=32", "-dir=/data"},
			WaitingFor:   wait.ForHTTP("/").WithPort(seaweedS3Port).WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, seaweedS3Port)
	require.NoError(t, err)
	store := storage.NewS3Storage(storage.S3Config{Endpoint: fmt.Sprintf("http://%s:%s", host, port.Port()), AccessKey: "any", SecretKey: "any", Region: "us-east-1"})
	require.NoError(t, store.EnsureBucket(ctx, objectsBucket))
	return store
}

// newGoJudge starts the project's real sandbox image and returns its gRPC client and file transport.
func newGoJudge(t *testing.T) (*sandbox.Client, *sandbox.FileTransport) {
	t.Helper()
	ctx := context.Background()
	request := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{Context: "../../../deploy/common/judge", Dockerfile: "Dockerfile", KeepImage: true},
		ExposedPorts:   []string{goJudgeHTTPPort, goJudgeGRPCPort}, Privileged: true,
		Cmd:        []string{"-enable-grpc", "-grpc-addr=0.0.0.0:5051", "-http-addr=0.0.0.0:5050", "-dir=/tmp/gate-files", "-parallelism=2"},
		WaitingFor: wait.ForHTTP("/version").WithPort(goJudgeHTTPPort).WithStartupTimeout(e2eSandboxTimout)}
	// CI prebuilds the image; locally it is built from the project's Dockerfile.
	if image := os.Getenv("GOJUDGE_TEST_IMAGE"); image != "" {
		request.FromDockerfile = testcontainers.FromDockerfile{}
		request.Image = image
	}
	judge, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = judge.Terminate(context.Background()) })
	host, err := judge.Host(ctx)
	require.NoError(t, err)
	httpPort, err := judge.MappedPort(ctx, goJudgeHTTPPort)
	require.NoError(t, err)
	grpcPort, err := judge.MappedPort(ctx, goJudgeGRPCPort)
	require.NoError(t, err)
	client, err := sandbox.NewClient(host + ":" + grpcPort.Port())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	files, err := sandbox.NewFileTransport("http://"+host+":"+httpPort.Port(), &http.Client{Timeout: time.Minute})
	require.NoError(t, err)
	return client, files
}

func setupE2EEnvironment(t *testing.T) (context.Context, *pgxpool.Pool, jetstream.JetStream) {
	t.Helper()
	ctx := context.Background()
	pgContainer, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("gate_e2e"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(context.Background()) })
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	dbPool, err := pkg.NewPostgresDB(connStr)
	require.NoError(t, err)
	t.Cleanup(dbPool.Close)
	_, b, _, _ := runtime.Caller(0)
	sqlDB, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	defer sqlDB.Close()
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, filepath.Join(filepath.Dir(b), "../../migrations")))
	js, _ := newNATSJetStream(t)
	require.NoError(t, pkg.EnsureSubmissionsStream(ctx, js))
	return ctx, dbPool, js
}

func TestJudgeFullFlowE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	ctx, dbPool, js := setupE2EEnvironment(t)
	store := newSeaweedBackedS3Storage(t)
	client, files := newGoJudge(t)
	cfg, err := sandbox.LoadConfig("../../../languages.yaml")
	require.NoError(t, err)

	orgsRepo := pgstore.NewOrganizationsRepo(dbPool)
	usersRepo := pgstore.NewUsersRepo(dbPool)
	contestsRepo := pgstore.NewContestsRepo(dbPool)
	submissionsRepo := pgstore.NewSubmissionsRepo(dbPool)
	drafts := pgstore.NewProblemDraftsRepo(dbPool, problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10}, time.Minute)

	userID := uuid.New()
	email := "test@example.com"
	require.NoError(t, usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{Id: userID, Username: "testuser", Email: &email, PasswordHash: "hashed_password", Role: accessmodel.UserRoleUser}))
	org, err := orgsRepo.CreateOrganization(ctx, &organizationsmodel.CreateOrganizationInput{Login: "test-org", Name: "Test Org", CreatorID: userID})
	require.NoError(t, err)
	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser})

	// Draft: generated inputs (1 2 and 3 2), answers produced by the main solution,
	// explicit weights 40.00 and 60.00 in one "complete" group.
	problemID := uuid.New()
	require.NoError(t, dbfixtures.InsertProblem(ctx, dbPool, dbfixtures.ProblemSpec{ID: problemID, OrganizationID: org.ID, OwnerID: &userID, Visibility: "public", Title: "Sum of Two Numbers", ShortName: "sum-problem"}))
	d, err := drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)
	checker, generator, solution, group := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	d.Files = map[string]string{
		"checkers/check.cpp": "#include <fstream>\nint main(int n,char**v){long long a,b;std::ifstream out(v[2]),ans(v[3]);return !(out>>a) || !(ans>>b) || a!=b;}",
		"generators/gen.py":  "import sys\nprint(sys.argv[1], sys.argv[2])\n",
		"solutions/main.cpp": "#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<'\\n';}",
		"statements/en.md":   "Print a+b.",
	}
	d.Components = []problemsmodel.DraftComponent{
		{ID: checker, Role: "checker", Path: "checkers/check.cpp", Language: "cpp"},
		{ID: generator, Role: "generator", Path: "generators/gen.py", Language: "python"},
		{ID: solution, Role: "solution", Path: "solutions/main.cpp", Language: "cpp"},
	}
	d.CheckerID, d.MainSolutionID = &checker, &solution
	d.Subtasks = []problemsmodel.DraftSubtask{{ID: group, Ordinal: 1, Name: "All", Policy: "complete"}}
	seed := int64(1)
	test := func(ordinal int, weight points.Value, a, b string) problemsmodel.DraftTest {
		return problemsmodel.DraftTest{ID: uuid.New(), Ordinal: ordinal, SubtaskID: group, Points: weight,
			Input:  problemsmodel.DraftInput{Mode: "generator", ComponentID: &generator, Args: []string{a, b}, Seed: &seed},
			Answer: problemsmodel.DraftAnswer{Mode: "generated", ComponentID: &solution}}
	}
	d.Tests = []problemsmodel.DraftTest{test(1, 40*points.Unit, "1", "2"), test(2, 60*points.Unit, "3", "2")}
	saved, err := drafts.SaveDraft(ctx, mutationKey(org.ID, "edit", d), d)
	require.NoError(t, err)
	require.NotEmpty(t, saved.Body)
	d, err = drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)

	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 10 * time.Second, Memory: 256 << 20, Processes: 50, Output: 32 << 20, Diagnostic: 4096}
	compile := aux
	compile.CPU, compile.Wall, compile.Memory, compile.Processes = 10*time.Second, 20*time.Second, 512<<20, 150
	limits := problemsmodel.BuildLimits{DiskBytes: 256 << 20, FileCount: 100, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: compile, Auxiliary: aux}
	toolchain := problemsmodel.Toolchain{Digest: toolchainDigest, Architecture: "amd64", ABI: "linux-gnu", Languages: cfg.Langs}
	_, err = drafts.EnqueueBuild(ctx, mutationKey(org.ID, "build", d.Revision), problemID, d.Revision, problemsmodel.BuildOptions{Toolchain: toolchain, Limits: limits, MaxAttempts: 3, Duration: 2 * time.Minute, Lease: time.Minute})
	require.NoError(t, err)
	build, err := drafts.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	work := diskTempDir(t)
	pipeline := builder.Pipeline{Repo: drafts, Store: store, Bucket: objectsBucket, TempDir: work, Client: client, Files: files, Environment: toolchain, UploadTTL: time.Minute, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	packageID, err := pipeline.Execute(ctx, build)
	require.NoError(t, err)

	judgeUC := judging.NewJudgeUseCase(pgstore.NewJudgeJobs(dbPool), pgstore.NewJudgeTransactions(dbPool), submissionsRepo,
		&judging.PackageLoader{Store: store, Bucket: objectsBucket, TempDir: diskTempDir(t), Packages: drafts, Limits: limits},
		&judging.PackageJudge{Client: client, Files: files, Environment: toolchain, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		judgeworker.NewEventPublisher(js))

	contestID := uuid.New()
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	require.NoError(t, contestsRepo.CreateContest(ctx, &contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, OwnerID: &userID, Title: "E2E Contest", Login: "e2e-contest", Description: "E2E", Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &start, EndTime: &end, Settings: map[string]interface{}{}}))
	require.NoError(t, contestsRepo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: problemID, PackageId: packageID}))
	require.NoError(t, contestsRepo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: userID, Role: contestsmodel.ContestRoleParticipant}))

	judge := func(t *testing.T, problem uuid.UUID, language submissionsmodel.LanguageName, source string) submissionsmodel.Submission {
		t.Helper()
		id, err := submissionsRepo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{Solution: source, ProblemId: problem, ContestId: contestID, UserId: userID, Language: language})
		require.NoError(t, err)
		job, err := pgstore.NewJudgeJobs(dbPool).ScheduleJob(ctx, id)
		require.NoError(t, err)
		require.NoError(t, judgeUC.JudgeSubmission(ctx, job.Ref()))
		sub, err := submissionsRepo.GetSubmission(ctx, id)
		require.NoError(t, err)
		return sub
	}

	t.Run("CorrectSolutionScoresEveryWeight", func(t *testing.T) {
		sub := judge(t, problemID, submissionsmodel.Python, "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.Accepted, sub.State)
		assert.Equal(t, 100*points.Unit, sub.Score)
	})
	t.Run("WrongSolutionIsRejected", func(t *testing.T) {
		sub := judge(t, problemID, submissionsmodel.Python, "print(42)")
		assert.Equal(t, submissionsmodel.GotWA, sub.State)
		assert.Equal(t, points.Value(0), sub.Score)
		require.NotNil(t, sub.FailedTest)
		assert.EqualValues(t, 1, *sub.FailedTest)
	})
	t.Run("UnreadablePackageIsInternalError", func(t *testing.T) {
		// A Package whose archive object was never uploaded cannot be loaded.
		brokenProblem := uuid.New()
		require.NoError(t, dbfixtures.InsertProblem(ctx, dbPool, dbfixtures.ProblemSpec{ID: brokenProblem, OrganizationID: org.ID, OwnerID: &userID, Visibility: "public", Title: "Broken", ShortName: "broken-problem"}))
		brokenPackage, err := dbfixtures.InsertPackage(ctx, dbPool, brokenProblem, org.ID, time.Time{})
		require.NoError(t, err)
		require.NoError(t, contestsRepo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: brokenProblem, PackageId: brokenPackage}))
		sub := judge(t, brokenProblem, submissionsmodel.Python, "print(1)")
		assert.Equal(t, submissionsmodel.GotIE, sub.State)
	})
}

func mutationKey(org uuid.UUID, operation string, body any) problemsmodel.MutationKey {
	data, _ := json.Marshal(body)
	sum := sha256.Sum256(data)
	return problemsmodel.MutationKey{OrganizationID: org, Operation: operation, Key: uuid.NewString(), Fingerprint: hex.EncodeToString(sum[:]), Token: uuid.New()}
}
