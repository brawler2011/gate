//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/worker/judge"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveStrategySmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	sandboxInst := newWorkshopSandbox(t)

	js, natsConn := newNATSJetStream(t)
	defer natsConn.Close()
	require.NoError(t, pkg.EnsureSubmissionsStream(ctx, js))

	eventPublisher := judge.NewEventPublisher(js)

	// Compile the interactor
	binary, err := sandboxInst.Compile(ctx, []byte(smokeInteractorSource()), "cpp", nil)
	require.NoError(t, err)
	require.NotEmpty(t, binary.PrimaryFileID)

	// Construct mock gfmt package directory
	tempPkgDir := t.TempDir()
	probYaml := `format_version: "1.0"
title: "interactive-smoke"
type: "interactive"
limits:
  time_ms: 1000
  memory_mb: 256
subtasks:
  samples:
    points: 100
    policy: "complete"
    tests:
      - manual: "01.in"
`
	err = os.WriteFile(filepath.Join(tempPkgDir, "problem.yaml"), []byte(probYaml), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tempPkgDir, "tests"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempPkgDir, "tests", "01.in"), []byte("7\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempPkgDir, "tests", "01.out"), []byte("7\n"), 0644)
	require.NoError(t, err)

	gfmtPkg, err := gfmt.OpenPackage(tempPkgDir)
	require.NoError(t, err)

	compiledComponents := map[string]sandbox.Executable{
		"interactor": binary,
	}

	strategy := judge.NewInteractiveStrategy(
		sandboxInst,
		eventPublisher,
		gfmtPkg,
		compiledComponents,
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

	if verdict.State != models.Accepted {
		t.Logf("Verdict Message: %s", verdict.Message)
	}
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

    if (argc >= 3) {
        std::ofstream logFile(argv[2]);
        logFile << "ok" << std::endl;
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
