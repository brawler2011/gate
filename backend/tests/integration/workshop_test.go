//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkshopE2E tests the complete workshop workflow
func TestWorkshopE2E(t *testing.T) {
	// Skip if go-judge is not available
	goJudgeAddr := os.Getenv("GOJUDGE_GRPC_ADDR")
	if goJudgeAddr == "" {
		goJudgeAddr = "localhost:5051"
	}

	// Create temp directory for repos
	tempDir, err := os.MkdirTemp("", "workshop-e2e-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize VCS service
	vcsService := vcs.NewGoGitService(tempDir)

	// Initialize Sandbox client
	sandboxClient, err := sandbox.NewClient(sandbox.ClientConfig{
		Protocol: sandbox.ProtocolGRPC,
		BaseURL:  goJudgeAddr,
	})
	if err != nil {
		t.Skip("go-judge not available, skipping integration test")
	}
	defer sandboxClient.Close()

	sandboxOrch := sandbox.NewOrchestrator(sandboxClient)

	// Create mock repositories (simplified for testing)
	// In real test, you'd use actual DB or mocks
	problemsRepo := &mockProblemsRepo{}
	txManager := &mockTxManager{}

	// Initialize WorkshopUseCase
	workshopUC := usecase.NewWorkshopUseCase(
		problemsRepo,
		vcsService,
		sandboxOrch,
		txManager,
	)

	ctx := context.Background()
	problemID := uuid.New()

	// Step 1: Initialize workshop
	t.Run("InitWorkshop", func(t *testing.T) {
		err := workshopUC.InitProblemWorkshop(ctx, problemID, "A+B Problem")
		require.NoError(t, err)

		// Verify repo exists
		assert.True(t, vcsService.RepoExists(ctx, problemID))

		// Verify manifest exists
		manifest, err := vcsService.LoadManifest(ctx, problemID)
		require.NoError(t, err)
		assert.Equal(t, "A+B Problem", manifest.Statements["en"].Title)
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

	// Step 5: Commit changes
	t.Run("CommitChanges", func(t *testing.T) {
		commitSHA, err := workshopUC.CommitChanges(ctx, problemID, "Add checker and solution", "Test User", "test@example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, commitSHA)

		// Verify commit history
		commits, err := workshopUC.GetCommitHistory(ctx, problemID, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(commits), 2) // Initial + our commit
	})

	// Step 6: Verify git history
	t.Run("VerifyHistory", func(t *testing.T) {
		commits, err := workshopUC.GetCommitHistory(ctx, problemID, 10)
		require.NoError(t, err)

		// Find our commit
		found := false
		for _, commit := range commits {
			if commit.Message == "Add checker and solution" {
				found = true
				assert.Equal(t, "Test User", commit.Author)
				break
			}
		}
		assert.True(t, found, "commit should be in history")
	})
}

// Mock implementations for testing
type mockProblemsRepo struct{}

func (m *mockProblemsRepo) UpdateProblem(ctx context.Context, id uuid.UUID, update interface{}) error {
	return nil
}

type mockTxManager struct{}

func (m *mockTxManager) WithTx(ctx context.Context, fn func(context.Context, interface{}) error) error {
	return fn(ctx, nil)
}
