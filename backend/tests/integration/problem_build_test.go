//go:build integration
// +build integration

package integration

import (
	"io"
	"log/slog"
	"time"

	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/diskcache"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

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
	// Without a heartbeat, the lease must cover the whole build under parallel test load.
	build, err := s.problemDrafts.ClaimBuild(s.ctx, 5*time.Minute)
	s.Require().NoError(err)
	s.Require().Equal(accepted.BuildID, build.ID)
	pipeline := builder.Pipeline{Repo: s.problemDrafts, Store: s.objectStore, Bucket: s.bucket, TempDir: diskTempDir(s.T()), Client: client, Files: files, Languages: &sandbox.Config{Langs: options.Languages}, Programs: sandbox.NewProgramCache(diskCache(s.T()), client, files, sandbox.Image{Config: &sandbox.Config{Langs: options.Languages}}), Parallelism: judgeParallelism(s.T()), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	packageID, err := pipeline.Execute(s.ctx, build)
	s.Require().NoError(err)
	return packageID
}

func testLanguages(t *testing.T) *sandbox.Config {
	t.Helper()
	cfg, err := sandbox.LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	return cfg
}

func testBuildOptions(t *testing.T) problemsmodel.BuildOptions {
	t.Helper()
	cfg, err := sandbox.LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 10 * time.Second, Memory: 256 << 20, Processes: 50, Output: 32 << 20, Diagnostic: 4096}
	compile := aux
	compile.CPU, compile.Wall, compile.Memory, compile.Processes = 20*time.Second, 60*time.Second, 512<<20, 150
	return problemsmodel.BuildOptions{
		Languages:   cfg.Langs,
		Limits:      problemsmodel.BuildLimits{DiskBytes: 256 << 20, FileCount: 1000, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: compile, Auxiliary: aux},
		MaxAttempts: 3, Duration: 10 * time.Minute, Lease: time.Minute,
	}
}

func diskTempDir(t *testing.T) string {
	t.Helper()
	return dbfixtures.DiskTempDir(t)
}

func diskCache(t *testing.T) *diskcache.Cache {
	t.Helper()
	return dbfixtures.DiskCache(t)
}
