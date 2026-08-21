//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/internal/worker/judge"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
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
	seaweedS3Port = "8333/tcp"
)

func newSeaweedBackedS3Storage(t *testing.T) (context.Context, storage.Storage) {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "chrislusf/seaweedfs:3.77_full",
			ExposedPorts: []string{seaweedS3Port},
			Cmd:          []string{"server", "-s3", "-filer", "-ip.bind=0.0.0.0", "-volume.max=100", "-dir=/data"},
			WaitingFor:   wait.ForListeningPort(seaweedS3Port).WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, seaweedS3Port)
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	store := storage.NewS3Storage(storage.S3Config{
		Endpoint:  endpoint,
		AccessKey: "any",
		SecretKey: "any",
		Region:    "us-east-1",
	})

	for _, bucket := range []string{"problem-packages", "problem-workspaces"} {
		err := store.EnsureBucket(ctx, bucket)
		require.NoError(t, err)
	}

	return ctx, store
}

func setupE2EEnvironment(t *testing.T) (
	context.Context,
	*pgxpool.Pool,
	jetstream.JetStream,
	storage.Storage,
	*sandbox.Sandbox,
) {
	t.Helper()
	ctx := context.Background()

	// 1. Postgres
	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("gate_e2e"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(10*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(context.Background())
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	dbPool, err := pkg.NewPostgresDB(connStr)
	require.NoError(t, err)
	t.Cleanup(func() {
		dbPool.Close()
	})

	// Run migrations
	_, b, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(b), "../../migrations")
	sqlDB, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, migrationsPath))

	// 2. NATS JetStream
	js, natsConn := newNATSJetStream(t)
	require.NoError(t, pkg.EnsureSubmissionsStream(ctx, js))
	_ = natsConn

	// 3. SeaweedFS S3
	_, s3Store := newSeaweedBackedS3Storage(t)

	// 4. Go-Judge Sandbox
	sb := newWorkshopSandbox(t)

	return ctx, dbPool, js, s3Store, sb
}

func TestJudgeFullFlowE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, dbPool, js, s3Store, sb := setupE2EEnvironment(t)

	// Repositories
	txManager := pg.NewTransactor(dbPool)
	orgsRepo := pg.NewOrganizationsRepo(dbPool)
	usersRepo := pg.NewUsersRepo(dbPool)
	problemsRepo := pg.NewProblemsRepo(dbPool)
	contestsRepo := pg.NewContestsRepo(dbPool)
	submissionsRepo := pg.NewSubmissionsRepo(dbPool)
	packagesRepo := pg.NewPackagesRepo(dbPool)

	// Storage & UseCases
	workspaceStorage := usecase.NewWorkspaceStorage(s3Store, "problem-workspaces")
	workshopUC := usecase.NewWorkshopUseCase(problemsRepo, workspaceStorage, sb, txManager)
	publishUC := usecase.NewProblemPublishUseCase(problemsRepo, packagesRepo, workspaceStorage, s3Store, "problem-packages")
	contestsUC := usecase.NewContestsUseCase(contestsRepo)

	eventPublisher := judge.NewEventPublisher(js)
	tempDir := t.TempDir()
	judgeUC := usecase.NewJudgeUseCase(
		submissionsRepo,
		packagesRepo,
		contestsUC,
		s3Store,
		"problem-packages",
		tempDir,
		sb,
		eventPublisher,
	)

	// 1. Create Organization & User
	userID := uuid.New()
	err := usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		Role:         models.UserRoleUser,
	})
	require.NoError(t, err)

	org, err := orgsRepo.CreateOrganization(ctx, &models.CreateOrganizationInput{
		Login:     "test-org",
		Name:      "Test Org",
		CreatorID: userID,
	})
	require.NoError(t, err)

	// 2. Create Problem in Workshop
	problemID := uuid.New()
	err = problemsRepo.CreateProblem(ctx, &models.CreateProblemParams{
		ID:             problemID,
		OrganizationID: org.ID,
		OwnerID:        &userID,
		Visibility:     "public",
		Title:          "Sum of Two Numbers",
		ShortName:      "sum-problem",
	})
	require.NoError(t, err)

	err = workshopUC.InitProblemWorkshop(ctx, problemID, "Sum of Two Numbers")
	require.NoError(t, err)

	// Add test cases (01: 1 2 -> 3; 02: 3 2 -> 5)
	err = workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    userID,
		Path:      "tests/01.in",
		Content:   []byte("1 2\n"),
	})
	require.NoError(t, err)
	err = workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    userID,
		Path:      "tests/01.out",
		Content:   []byte("3\n"),
	})
	require.NoError(t, err)

	err = workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    userID,
		Path:      "tests/02.in",
		Content:   []byte("3 2\n"),
	})
	require.NoError(t, err)
	err = workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    userID,
		Path:      "tests/02.out",
		Content:   []byte("5\n"),
	})
	require.NoError(t, err)

	// 3. Publish Problem to SeaweedFS S3
	pubRes, err := publishUC.PublishProblem(ctx, problemID)
	require.NoError(t, err)
	require.NotNil(t, pubRes)

	// 4. Create Contest
	contestID := uuid.New()
	err = contestsRepo.CreateContest(ctx, &models.CreateContestParams{
		ID:             contestID,
		OrganizationID: org.ID,
		OwnerID:        &userID,
		Title:          "E2E Contest",
		Login:          "e2e-contest",
		Description:    "E2E Contest for judging flow",
		Visibility:     models.ContestVisibilityPublic,
		Settings:       make(map[string]interface{}),
		AccessPolicy:   models.DefaultContestAccessPolicy(),
	})
	require.NoError(t, err)

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	err = contestsRepo.UpdateContest(ctx, models.ContestUpdateParams{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	require.NoError(t, err)

	err = contestsRepo.CreateContestProblem(ctx, models.ContestProblemCreation{
		ContestId: contestID,
		ProblemId: problemID,
		PackageId: pubRes.PackageID,
	})
	require.NoError(t, err)

	err = contestsRepo.CreateContestMember(ctx, &models.CreateContestMemberParams{
		ContestId: contestID,
		UserId:    userID,
		Role:      models.ContestRoleParticipant,
	})
	require.NoError(t, err)

	// 5. Test Correct Python Solution -> Accepted (200)
	t.Run("PythonCorrectSolution_Accepted", func(t *testing.T) {
		subID, err := submissionsRepo.CreateSubmission(ctx, &models.SubmissionCreation{
			Solution:  "print(sum(map(int, input().split())))",
			ProblemId: problemID,
			ContestId: contestID,
			UserId:    userID,
			Language:  models.Python,
		})
		require.NoError(t, err)

		err = judgeUC.JudgeSubmission(ctx, subID)
		require.NoError(t, err)

		sub, err := submissionsRepo.GetSubmission(ctx, subID)
		require.NoError(t, err)
		assert.Equal(t, models.Accepted, sub.State, "Expected Accepted (200) for correct Python solution")
		assert.Equal(t, int32(100), sub.Score)
	})

	// 6. Test Wrong Python Solution -> Wrong Answer (106)
	t.Run("PythonWrongSolution_WrongAnswer", func(t *testing.T) {
		subID, err := submissionsRepo.CreateSubmission(ctx, &models.SubmissionCreation{
			Solution:  "print(42)",
			ProblemId: problemID,
			ContestId: contestID,
			UserId:    userID,
			Language:  models.Python,
		})
		require.NoError(t, err)

		err = judgeUC.JudgeSubmission(ctx, subID)
		require.NoError(t, err)

		sub, err := submissionsRepo.GetSubmission(ctx, subID)
		require.NoError(t, err)
		assert.Equal(t, models.GotWA, sub.State, "Expected Wrong Answer (106) for incorrect solution")
		assert.Equal(t, int32(0), sub.Score)
	})

	// 7. Test Internal Error on Corrupted / Missing Package -> GotIE (107)
	t.Run("InternalError_GotIE", func(t *testing.T) {
		// Create a problem with invalid package hash
		brokenProblemID := uuid.New()
		err = problemsRepo.CreateProblem(ctx, &models.CreateProblemParams{
			ID:             brokenProblemID,
			OrganizationID: org.ID,
			OwnerID:        &userID,
			Visibility:     "public",
			Title:          "Broken Problem",
			ShortName:      "broken-problem",
		})
		require.NoError(t, err)

		brokenPackageID := uuid.New()
		_, err = packagesRepo.CreatePackage(ctx, &models.CreatePackageParams{
			ID:             brokenPackageID,
			ProblemID:      brokenProblemID,
			OrganizationID: org.ID,
			PackageHash:    "0000000000000000000000000000000000000000000000000000000000000000",
			Status:         "ready",
		})
		require.NoError(t, err)

		err = contestsRepo.CreateContestProblem(ctx, models.ContestProblemCreation{
			ContestId: contestID,
			ProblemId: brokenProblemID,
			PackageId: brokenPackageID,
		})
		require.NoError(t, err)

		subID, err := submissionsRepo.CreateSubmission(ctx, &models.SubmissionCreation{
			Solution:  "print(1)",
			ProblemId: brokenProblemID,
			ContestId: contestID,
			UserId:    userID,
			Language:  models.Python,
		})
		require.NoError(t, err)

		err = judgeUC.JudgeSubmission(ctx, subID)
		assert.Error(t, err)

		sub, err := submissionsRepo.GetSubmission(ctx, subID)
		require.NoError(t, err)
		assert.Equal(t, models.GotIE, sub.State, "Expected Internal Error (107) when package cannot be loaded")
	})
}
