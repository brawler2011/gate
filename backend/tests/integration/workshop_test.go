//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	workshopGoJudgeGRPCPort                  = "5051/tcp"
	workshopGoJudgeStartupTimeout            = 120 * time.Second
	workshopGoJudgeProbeRetryDelay           = 1500 * time.Millisecond
	workshopGoJudgeProbeCallTimeout          = 8 * time.Second
	workshopGoJudgeContainerTerminationGrace = 15 * time.Second
)

// TestWorkshopE2E tests the complete workshop workflow
func TestWorkshopE2E(t *testing.T) {
	sandboxOrch := newWorkshopSandboxOrchestrator(t)

	// Create temp directory for repos
	tempDir, err := os.MkdirTemp("", "workshop-e2e-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize VCS service
	vcsService := vcs.NewGoGitService(tempDir)
	ctx := context.Background()
	problemID := uuid.New()

	// Create mock repositories (simplified for testing)
	problemsRepo := newMockProblemsRepo()
	problemsRepo.problems[problemID] = models.Problem{ID: problemID, Title: "A+B Problem"}
	txManager := &mockTxManager{}

	// Initialize WorkshopUseCase
	workshopUC := usecase.NewWorkshopUseCase(
		problemsRepo,
		vcsService,
		sandboxOrch,
		txManager,
	)

	// Step 1: Initialize workshop
	t.Run("InitWorkshop", func(t *testing.T) {
		err := workshopUC.InitProblemWorkshop(ctx, problemID, "A+B Problem")
		require.NoError(t, err)

		files, err := vcsService.ListAllFiles(ctx, problemID)
		require.NoError(t, err)
		assert.NotEmpty(t, files)

		// Verify manifest exists
		manifestBytes, err := workshopUC.ReadProblemFile(ctx, problemID, "manifest.json")
		require.NoError(t, err)

		var manifest problemformat.ProblemManifest
		require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
		assert.Equal(t, "A+B Problem", manifest.Statement.Title)
	})

	// Step 2: Upload checker
	t.Run("UploadChecker", func(t *testing.T) {
		checkerCode := `#include "testlib.h"
int main(int argc, char* argv[]) {
    registerTestlibCmd(argc, argv);
    int ja = ans.readInt();
    int pa = ouf.readInt();
    if (ja != pa) {
        quitf(_wa, "expected %d, found %d", ja, pa);
    }
    quitf(_ok, "answer is correct");
}
`
		err := workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
			ProblemID: problemID,
			UserID:    uuid.Nil,
			Path:      "checkers/checker.cpp",
			Content:   []byte(checkerCode),
		})
		require.NoError(t, err)
	})

	// Step 3: Upload solution
	t.Run("UploadSolution", func(t *testing.T) {
		solutionCode := `#include <iostream>
using namespace std;
int main() {
    int a, b;
    cin >> a >> b;
    cout << a + b << endl;
    return 0;
}
`
		err := workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
			ProblemID: problemID,
			UserID:    uuid.Nil,
			Path:      "solutions/main.cpp",
			Content:   []byte(solutionCode),
		})
		require.NoError(t, err)
	})

	// Step 4: Test solution
	t.Run("TestSolution", func(t *testing.T) {
		report, err := workshopUC.TestSolution(ctx, models.TestSolutionRequest{
			ProblemID:    problemID,
			SolutionPath: "solutions/main.cpp",
			TestNumbers:  []int{1}, // Test only sample test
			UserID:       uuid.Nil,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, report.TotalTests)

		// Check if test passed
		if len(report.Results) > 0 {
			t.Logf("Test result: %s - %s", report.Results[0].Verdict, report.Results[0].Message)
		}
	})

	// Step 5: Verify workspace files were written
	t.Run("VerifyWorkspaceFiles", func(t *testing.T) {
		files, err := vcsService.ListAllFiles(ctx, problemID)
		require.NoError(t, err)
		assert.Contains(t, files, "checkers/checker.cpp")
		assert.Contains(t, files, "solutions/main.cpp")
		assert.Contains(t, files, "tests/01.in")
		assert.Contains(t, files, "tests/01.out")
	})
}

func newWorkshopSandboxOrchestrator(t *testing.T) *sandbox.Orchestrator {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		ExposedPorts: []string{workshopGoJudgeGRPCPort},
		Env: map[string]string{
			"ES_ENABLE_GRPC": "true",
			"ES_PARALLELISM": "2",
			"ES_PRE_FORK":    "1",
		},
		Privileged: true,
		WaitingFor: wait.ForListeningPort(workshopGoJudgeGRPCPort).
			WithStartupTimeout(workshopGoJudgeStartupTimeout),
	}

	if image := os.Getenv("GOJUDGE_TEST_IMAGE"); image != "" {
		req.Image = image
	} else {
		buildContext, dockerfileName, err := workshopGoJudgeDockerfileContext()
		require.NoError(t, err)

		req.FromDockerfile = testcontainers.FromDockerfile{
			Context:    buildContext,
			Dockerfile: dockerfileName,
			Repo:       "gate-workshop-gojudge",
			Tag:        "local",
			KeepImage:  false,
		}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), workshopGoJudgeContainerTerminationGrace)
		defer cancel()
		_ = container.Terminate(cleanupCtx)
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, workshopGoJudgeGRPCPort)
	require.NoError(t, err)

	client, err := sandbox.NewClient(sandbox.ClientConfig{
		Addr:    net.JoinHostPort(host, mappedPort.Port()),
		Timeout: 30 * time.Second,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = client.Close()
	})

	waitForWorkshopGoJudgeReady(t, ctx, client)

	return sandbox.NewOrchestrator(client)
}

func waitForWorkshopGoJudgeReady(t *testing.T, ctx context.Context, client *sandbox.Client) {
	t.Helper()

	deadline := time.Now().Add(workshopGoJudgeStartupTimeout)
	var lastErr error

	for time.Now().Before(deadline) {
		probeCtx, cancel := context.WithTimeout(ctx, workshopGoJudgeProbeCallTimeout)
		result, err := client.Compile(probeCtx, sandbox.CompileRequest{
			SourceCode: "int main(){return 0;}",
			Language:   "cpp17",
			SourceFile: "main.cpp",
			OutputFile: "main",
			Limits: sandbox.ResourceLimits{
				CPUTimeMs: 5000,
				MemoryMB:  256,
				ProcLimit: 10,
				StackMB:   64,
			},
			Dependencies: map[string]string{},
		})
		cancel()

		if err == nil && result != nil && result.Success {
			return
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("compile probe returned non-success status: %s", result.Stderr)
		}

		time.Sleep(workshopGoJudgeProbeRetryDelay)
	}

	require.FailNowf(t, "go-judge is not ready", "gRPC readiness probe failed: %v", lastErr)
}

func workshopGoJudgeDockerfileContext() (string, string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", fmt.Errorf("failed to resolve workshop_test.go path")
	}

	buildContext := filepath.Join(filepath.Dir(thisFile), "../../pkg/sandbox/testdata")
	return buildContext, "gojudge.Dockerfile", nil
}

// Mock implementations for testing
type mockProblemsRepo struct {
	manifests map[uuid.UUID][]byte
	problems  map[uuid.UUID]models.Problem
}

func newMockProblemsRepo() *mockProblemsRepo {
	return &mockProblemsRepo{
		manifests: make(map[uuid.UUID][]byte),
		problems:  make(map[uuid.UUID]models.Problem),
	}
}

func (m *mockProblemsRepo) CreateProblem(_ context.Context, _ *models.CreateProblemParams) error {
	return nil
}

func (m *mockProblemsRepo) CreateProblemMember(_ context.Context, _ *models.CreateProblemMemberParams) error {
	return nil
}

func (m *mockProblemsRepo) CreateProblemTests(_ context.Context, _ models.ProblemTests) error {
	return nil
}

func (m *mockProblemsRepo) DeleteProblem(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockProblemsRepo) DeleteProblemTests(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockProblemsRepo) GetProblemById(_ context.Context, id uuid.UUID) (models.Problem, error) {
	if problem, ok := m.problems[id]; ok {
		return problem, nil
	}
	return models.Problem{}, nil
}

func (m *mockProblemsRepo) GetProblemMember(_ context.Context, _, _ uuid.UUID) (models.ProblemMember, error) {
	return models.ProblemMember{}, nil
}

func (m *mockProblemsRepo) GetProblemTests(_ context.Context, _ uuid.UUID) ([]models.ProblemTest, error) {
	return nil, nil
}

func (m *mockProblemsRepo) GetProblemTeams(_ context.Context, _ uuid.UUID) ([]models.ProblemTeam, error) {
	return nil, nil
}

func (m *mockProblemsRepo) ListProblems(_ context.Context, _ *models.ProblemsFilter) ([]models.Problem, int32, error) {
	return nil, 0, nil
}

func (m *mockProblemsRepo) UpdateProblem(_ context.Context, id uuid.UUID, update *models.ProblemUpdate) error {
	problem := m.problems[id]
	if update != nil && update.Title != nil {
		problem.Title = *update.Title
	}
	m.problems[id] = problem
	return nil
}

func (m *mockProblemsRepo) UpdateProblemLimits(_ context.Context, _ uuid.UUID, _, _ int) error {
	return nil
}

func (m *mockProblemsRepo) GetProblemManifest(_ context.Context, id uuid.UUID) ([]byte, error) {
	return m.manifests[id], nil
}

func (m *mockProblemsRepo) UpdateProblemManifest(_ context.Context, id uuid.UUID, manifest []byte) error {
	m.manifests[id] = append([]byte(nil), manifest...)
	return nil
}

type mockTxManager struct{}

func (m *mockTxManager) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	return fn(ctx, nil)
}
