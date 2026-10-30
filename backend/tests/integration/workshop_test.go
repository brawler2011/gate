//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"gopkg.in/yaml.v3"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	sandboxInst := newWorkshopSandbox(t)

	// Initialize Storage-backed Workspace service
	store := storage.NewLocalStorage(t.TempDir())
	workspaceStorage := usecase.NewWorkspaceStorage(store, "integration-workshop", newMockProblemsRepo())
	ctx := context.Background()
	problemID := uuid.New()

	// Create mock repositories (simplified for testing)
	problemsRepo := newMockProblemsRepo()
	problemsRepo.problems[problemID] = models.Problem{ID: problemID, Title: "A+B Problem"}

	// Initialize WorkshopUseCase
	workshopUC := usecase.NewWorkshopUseCase(workspaceStorage, sandboxInst)

	// Step 1: Initialize workshop
	t.Run("InitWorkshop", func(t *testing.T) {
		err := workshopUC.InitProblemWorkshop(ctx, problemID, "A+B Problem")
		require.NoError(t, err)

		files, err := workspaceStorage.ListAllFiles(ctx, problemID)
		require.NoError(t, err)
		assert.NotEmpty(t, files)

		// Verify manifest exists
		manifest, err := workshopUC.GetManifest(ctx, problemID)
		require.NoError(t, err)
		assert.Equal(t, "A+B Problem", manifest.Statement.Title)
	})

	// Step 2: Upload checker and its dependencies
	t.Run("UploadChecker", func(t *testing.T) {
		// Upload testlib.h dependency to lib/
		testlibContent, err := os.ReadFile("../../pkg/testdata/gfmt/a-plus-b/lib/testlib.h")
		require.NoError(t, err)

		err = workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
			ProblemID: problemID,
			UserID:    uuid.Nil,
			Path:      "lib/testlib.h",
			Content:   testlibContent,
		})
		require.NoError(t, err)

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
		err = workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
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
		// Verify checker and solution exists in files
		files, err := workspaceStorage.ListAllFiles(ctx, problemID)
		require.NoError(t, err)
		assert.Contains(t, files, "checkers/checker.cpp")
		assert.Contains(t, files, "solutions/main.cpp")
		assert.Contains(t, files, "tests/01.in")
		assert.Contains(t, files, "tests/01.out")

		// First we must compile the checker component
		compRes, err := workshopUC.CompileProblemComponent(ctx, models.CompileComponentRequest{
			ProblemID:     problemID,
			ComponentType: "checker",
		})
		require.NoError(t, err)
		require.True(t, compRes.Success, compRes.CompileLog)

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
			assert.Equal(t, "OK", report.Results[0].Verdict)
		}
	})

	// Step 5: Verify workspace files were written
	t.Run("VerifyWorkspaceFiles", func(t *testing.T) {
		files, err := workspaceStorage.ListAllFiles(ctx, problemID)
		require.NoError(t, err)
		assert.Contains(t, files, "checkers/checker.cpp")
		assert.Contains(t, files, "solutions/main.cpp")
		assert.Contains(t, files, "tests/01.in")
		assert.Contains(t, files, "tests/01.out")
	})
}

func TestWorkshopCheckerEvaluation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	sb := newWorkshopSandbox(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name         string
		checker      string
		output       string
		files        map[string]string
		deps         []models.Dependency
		compileFirst bool
		verdict      string
		wantError    string
	}{
		{
			name:    "checker accepts different text",
			checker: "#include <fstream>\nint main(int argc, char** argv) { int actual, expected; std::ifstream(argv[2]) >> actual; std::ifstream(argv[3]) >> expected; return actual == expected ? 0 : 1; }",
			output:  "03",
			verdict: "OK",
		},
		{name: "checker rejects identical text", checker: "int main(){return 1;}", output: "3", verdict: "WA"},
		{name: "checker reports failure", checker: "int main(){return 3;}", output: "3", verdict: "FAIL"},
		{name: "compiler diagnostics", checker: "#error checker_regression_diagnostic", output: "3", wantError: "checker_regression_diagnostic"},
		{
			name:    "relative dependency takes precedence",
			checker: "#include \"verdict.h\"\nint main(){return CHECKER_EXIT;}",
			output:  "3",
			files:   map[string]string{"checkers/verdict.h": "#define CHECKER_EXIT 1", "lib/verdict.h": "#define CHECKER_EXIT 0"},
			deps:    []models.Dependency{{Filename: "verdict.h"}},
			verdict: "WA",
		},
		{
			name:    "shared dependency",
			checker: "#include \"verdict.h\"\nint main(){return CHECKER_EXIT;}",
			output:  "3",
			files:   map[string]string{"lib/verdict.h": "#define CHECKER_EXIT 1"},
			deps:    []models.Dependency{{Filename: "verdict.h"}},
			verdict: "WA",
		},
		{
			name:         "compiled checker with nested shared dependency",
			checker:      "#include \"verdict.h\"\nint main(){return CHECKER_EXIT;}",
			output:       "3",
			files:        map[string]string{"lib/support/verdict.h": "#define CHECKER_EXIT 1"},
			compileFirst: true,
			verdict:      "WA",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problemID := uuid.New()
			repo := newMockProblemsRepo()
			manifest := models.ProblemManifest{
				TimeLimitMs:   1000,
				MemoryLimitMb: 256,
				FilesMetadata: []models.FileMetadata{{Type: "checker", Filename: "checkers/checker.cpp", Dependencies: tc.deps}},
			}
			config := usecase.ManifestAndTestsToGfmtProblem(&manifest, &models.TestsMetadata{Groups: []models.TestGroup{{Ordinal: 1, Name: "samples", Tests: [2]int{1, 1}}}, Tests: []models.TestCase{{Ordinal: 1, Filename: "01.in", Method: "manual"}}})
			config.Sources = make(map[string]gfmt.Source)
			for _, meta := range manifest.FilesMetadata {
				var deps []string
				for _, dep := range meta.Dependencies {
					deps = append(deps, dep.Filename)
				}
				if len(deps) > 0 {
					config.Sources[meta.Filename] = gfmt.Source{Dependencies: deps}
				}
			}
			data, err := yaml.Marshal(config)
			require.NoError(t, err)

			ws := usecase.NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "checker-evaluation", repo)
			files := map[string]string{
				"problem.yaml":         string(data),
				"tests/01.in":          "1 2\n",
				"tests/01.out":         "3\n",
				"solutions/main.cpp":   "#include <iostream>\nint main(){std::cout << \"" + tc.output + "\\n\";}",
				"checkers/checker.cpp": tc.checker,
			}
			for path, content := range tc.files {
				files[path] = content
			}
			for path, content := range files {
				require.NoError(t, ws.WriteFile(ctx, problemID, path, []byte(content)))
			}
			uc := usecase.NewWorkshopUseCase(ws, sb)
			if tc.compileFirst {
				compiled, err := uc.CompileProblemComponent(ctx, models.CompileComponentRequest{ProblemID: problemID, ComponentType: "checker"})
				require.NoError(t, err)
				require.True(t, compiled.Success, compiled.CompileError)
			}
			report, err := uc.TestSolution(ctx, models.TestSolutionRequest{ProblemID: problemID, SolutionPath: "solutions/main.cpp"})
			if tc.wantError != "" {
				require.ErrorContains(t, err, "failed to compile checker checkers/checker.cpp")
				require.ErrorContains(t, err, tc.wantError)
				require.Nil(t, report)
				return
			}
			require.NoError(t, err)
			require.Len(t, report.Results, 1)
			require.Equal(t, tc.verdict, report.Results[0].Verdict)
			require.Equal(t, 1, report.TotalTests)
			if tc.verdict == "OK" {
				require.Equal(t, 1, report.PassedTests)
				require.Zero(t, report.FailedTests)
			} else {
				require.Zero(t, report.PassedTests)
				require.Equal(t, 1, report.FailedTests)
			}
		})
	}
}

func newWorkshopSandbox(t *testing.T) *sandbox.Sandbox {
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
			// Keep compiler layers cached between isolated sandbox containers.
			KeepImage: true,
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

	client, err := sandbox.NewClient(net.JoinHostPort(host, mappedPort.Port()))
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = client.Close()
	})

	cfg, err := sandbox.LoadConfig("../../languages.yaml")
	require.NoError(t, err)

	sb := sandbox.NewSandbox(client, cfg)
	waitForWorkshopGoJudgeReady(t, ctx, sb)

	return sb
}

func waitForWorkshopGoJudgeReady(t *testing.T, ctx context.Context, sb *sandbox.Sandbox) {
	t.Helper()

	deadline := time.Now().Add(workshopGoJudgeStartupTimeout)
	var lastErr error

	for time.Now().Before(deadline) {
		probeCtx, cancel := context.WithTimeout(ctx, workshopGoJudgeProbeCallTimeout)
		_, err := sb.Compile(probeCtx, []byte("int main(){return 0;}"), "cpp", nil)
		cancel()

		if err == nil {
			return
		}
		lastErr = err

		time.Sleep(workshopGoJudgeProbeRetryDelay)
	}

	require.FailNowf(t, "go-judge is not ready", "gRPC readiness probe failed: %v", lastErr)
}

func workshopGoJudgeDockerfileContext() (string, string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", fmt.Errorf("failed to resolve workshop_test.go path")
	}

	buildContext := filepath.Join(filepath.Dir(thisFile), "../../../deploy/common/judge")
	return buildContext, "Dockerfile", nil
}

// Mock implementations for testing
type mockProblemsRepo struct {
	workspaceMu     sync.Mutex
	workspaceHashes map[uuid.UUID]string
	problems        map[uuid.UUID]models.Problem
}

func newMockProblemsRepo() *mockProblemsRepo {
	return &mockProblemsRepo{
		problems: make(map[uuid.UUID]models.Problem),
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

func (m *mockProblemsRepo) AddProblemTeam(_ context.Context, _, _ uuid.UUID, _ models.ProblemPermission) error {
	return nil
}

func (m *mockProblemsRepo) UpdateProblemTeamPermission(_ context.Context, _, _ uuid.UUID, _ models.ProblemPermission) error {
	return nil
}

func (m *mockProblemsRepo) RemoveProblemTeam(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (m *mockProblemsRepo) ListProblemMembers(_ context.Context, _ uuid.UUID) ([]models.ProblemMember, error) {
	return nil, nil
}

func (m *mockProblemsRepo) UpdateProblemMemberRole(_ context.Context, _, _ uuid.UUID, _ models.ProblemRole) error {
	return nil
}

func (m *mockProblemsRepo) RemoveProblemMember(_ context.Context, _, _ uuid.UUID) error {
	return nil
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

func (m *mockProblemsRepo) ListDashboardProblems(_ context.Context, _ uuid.UUID, _ int32) ([]models.DashboardProblem, error) {
	return nil, nil
}

func (m *mockProblemsRepo) GetWorkspaceHash(ctx context.Context, id uuid.UUID) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.workspaceMu.Lock()
	defer m.workspaceMu.Unlock()
	return m.workspaceHashes[id], nil
}
func (m *mockProblemsRepo) CommitWorkspace(ctx context.Context, id uuid.UUID, previousHash, hash string, metadata *models.WorkspaceMetadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.workspaceMu.Lock()
	defer m.workspaceMu.Unlock()
	if m.workspaceHashes[id] != previousHash {
		return pkg.ErrConflict
	}
	if m.workspaceHashes == nil {
		m.workspaceHashes = make(map[uuid.UUID]string)
	}
	m.workspaceHashes[id] = hash
	if metadata != nil {
		problem := m.problems[id]
		problem.Title = metadata.Title
		problem.TimeLimitMs = metadata.TimeLimitMs
		problem.MemoryLimitMb = metadata.MemoryLimitMb
		m.problems[id] = problem
	}
	return nil
}
