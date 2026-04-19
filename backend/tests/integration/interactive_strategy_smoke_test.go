//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/worker/judge"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveStrategySmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	orchestrator := newWorkshopSandboxOrchestrator(t)

	js, natsConn := newNATSJetStream(t)
	defer natsConn.Close()
	require.NoError(t, pkg.EnsureSubmissionsStream(ctx, js))

	eventPublisher := judge.NewEventPublisher(js)

	binary, err := orchestrator.CompileComponentFromSource(ctx, "interactor", smokeInteractorSource(), "cpp17", nil)
	require.NoError(t, err)
	require.True(t, binary.Success, binary.CompileLog)
	require.NotEmpty(t, binary.FileID)

	problemPkg := &problemformat.ProblemPackage{
		Manifest: problemformat.ProblemManifest{
			ProblemType:   "interactive",
			TimeLimitMs:   1000,
			MemoryLimitMb: 256,
		},
		TestsMetadata: problemformat.TestsMetadata{
			Tests: []problemformat.TestCase{{
				Ordinal:  1,
				Method:   "manual",
				IsSample: true,
			}},
		},
		TestCases: []problemformat.LoadedTestCase{{
			Ordinal: 1,
			Input:   []byte("7\n"),
			Output:  []byte("7\n"),
		}},
	}

	strategy := judge.NewInteractiveStrategy(
		orchestrator.Client(),
		eventPublisher,
		problemPkg,
		map[string]string{"interactor": binary.FileID},
	)

	verdict, err := strategy.Judge(
		ctx,
		uuid.New(),
		smokeSolutionSource(),
		models.Cpp,
		models.SubmissionEventMeta{
			Username:     "integration",
			ContestTitle: "integration",
			ProblemTitle: "interactive-smoke",
			Language:     models.Cpp,
			CreatedAt:    time.Now(),
		},
	)
	require.NoError(t, err)

	assert.Equal(t, models.Accepted, verdict.State)
	assert.Equal(t, int32(100), verdict.Score)
}

func smokeInteractorSource() string {
	return `#include <fstream>
#include <iostream>

int main(int argc, char* argv[]) {
    if (argc < 2) {
        return 3;
    }

    std::ifstream in(argv[1]);
    long long expected = 0;
    if (!(in >> expected)) {
        return 3;
    }

    std::cout << expected << std::endl;

    long long reply = 0;
    if (!(std::cin >> reply)) {
        return 1;
    }

    if (reply != expected) {
        return 1;
    }

    return 0;
}
`
}

func smokeSolutionSource() string {
	return `#include <iostream>

int main() {
    long long x = 0;
    if (!(std::cin >> x)) {
        return 0;
    }
    std::cout << x << std::endl;
    return 0;
}
`
}
