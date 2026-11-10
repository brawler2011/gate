//go:build integration
// +build integration

package integration

import (
	"io"
	"log/slog"
	"time"

	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// buildPackage drives the public build API and runs the build worker pipeline once
// against the real sandbox and object store, returning the published Package id.
func (s *IntegrationTestSuite) buildPackage(userID, problemID uuid.UUID) uuid.UUID {
	s.T().Helper()
	ctx := withTestUser(s.ctx, userID)
	draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problemID})
	s.Require().NoError(err)
	accepted, err := s.client.BuildProblemPackage(ctx, &corev1.BuildProblemPackageReq{ExpectedRevision: draft.Revision},
		corev1.BuildProblemPackageParams{ID: problemID, IdempotencyKey: uuid.NewString()})
	s.Require().NoError(err)
	client, files := newGoJudge(s.T())
	options := testBuildOptions(s.T())
	build, err := s.problemDrafts.ClaimBuild(s.ctx, time.Minute)
	s.Require().NoError(err)
	s.Require().Equal(accepted.BuildID, build.ID)
	pipeline := builder.Pipeline{Repo: s.problemDrafts, Store: s.objectStore, Bucket: objectsBucket, TempDir: diskTempDir(s.T()), Client: client, Files: files, Environment: options.Toolchain, UploadTTL: time.Minute, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	packageID, err := pipeline.Execute(s.ctx, build)
	s.Require().NoError(err)
	return packageID
}

// testBuildOptions are the build settings the HTTP suite and its worker share.
func testBuildOptions(t *testing.T) problemsmodel.BuildOptions {
	t.Helper()
	cfg, err := sandbox.LoadConfig("../../../languages.yaml")
	require.NoError(t, err)
	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 10 * time.Second, Memory: 256 << 20, Processes: 50, Output: 32 << 20, Diagnostic: 4096}
	compile := aux
	compile.CPU, compile.Wall, compile.Memory, compile.Processes = 10*time.Second, 20*time.Second, 512<<20, 150
	return problemsmodel.BuildOptions{
		Toolchain:   problemsmodel.Toolchain{Digest: toolchainDigest, Architecture: "amd64", ABI: "linux-gnu", Languages: cfg.Langs},
		Limits:      problemsmodel.BuildLimits{DiskBytes: 256 << 20, FileCount: 100, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: compile, Auxiliary: aux},
		MaxAttempts: 3, Duration: 2 * time.Minute, Lease: time.Minute,
	}
}

// diskTempDir returns a disk-backed working directory; the builder refuses tmpfs.
func diskTempDir(t *testing.T) string {
	t.Helper()
	return dbfixtures.DiskTempDir(t)
}
