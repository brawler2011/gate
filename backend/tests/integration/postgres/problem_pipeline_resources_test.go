//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type allowAll struct{}

func (allowAll) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return true, nil
}

type fixedProblem struct{ org uuid.UUID }

func (f fixedProblem) GetProblemById(_ context.Context, id uuid.UUID) (model.Problem, error) {
	return model.Problem{ID: id, OrganizationID: f.org}, nil
}

type pipelineHarness struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	repo     *postgres.ProblemDraftsRepo
	org      uuid.UUID
	draft    model.Draft
	services pipelineServices
	work     string
	limits   model.BuildLimits
	options  postgres.ProblemBuildOptions
	uploads  *problems.DraftUseCase
}

func newPipelineHarness(t *testing.T, objectBytes, diskBytes int64) *pipelineHarness {
	t.Helper()
	ctx, pool, repo, org, d := draftFixture(t)
	services := startPipelineServices(t)
	work := dbfixtures.DiskTempDir(t)
	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 20 * time.Second, Memory: 256 << 20, Processes: 50, Output: bytesize.Size(objectBytes), Diagnostic: 4096}
	compile := aux
	compile.CPU, compile.Wall, compile.Memory, compile.Processes = 10*time.Second, 30*time.Second, 512<<20, 150
	limits := model.BuildLimits{DiskBytes: bytesize.Size(diskBytes), FileCount: 100, ObjectBytes: bytesize.Size(objectBytes), TestOutputBytes: bytesize.Size(objectBytes), Compile: compile, Auxiliary: aux}
	options := postgres.ProblemBuildOptions{
		Toolchain: model.Toolchain{Digest: "sha256:" + strings.Repeat("a", 64), Architecture: "amd64", ABI: "linux-gnu", Languages: services.langs},
		Limits:    limits, MaxAttempts: 1, Duration: 5 * time.Minute, Lease: 3 * time.Second,
	}
	uploads := &problems.DraftUseCase{
		DraftQuotas: model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10},
		Repo:        repo, Problems: fixedProblem{org}, Permissions: allowAll{}, Store: services.store, Bucket: "objects",
		TempDir: work, UploadTTL: 5 * time.Minute, ObjectBytes: objectBytes, BuildOptions: options,
	}
	return &pipelineHarness{ctx: accessmodel.WithUser(ctx, accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}),
		pool: pool, repo: repo, org: org, draft: d, services: services, work: work, limits: limits, options: options, uploads: uploads}
}

func (h *pipelineHarness) pipeline() *builder.Pipeline {
	return &builder.Pipeline{Repo: h.repo, Store: h.services.store, Bucket: "objects", TempDir: h.work, Client: h.services.client,
		Files: h.services.files, Environment: h.options.Toolchain, UploadTTL: time.Minute, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func (h *pipelineHarness) save(t *testing.T, change func(*model.Draft)) {
	t.Helper()
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	change(&current)
	_, err = h.repo.SaveDraft(h.ctx, mutationKey(h.org, "edit", current), current)
	require.NoError(t, err)
}

// randomFile writes size bytes of incompressible data to a new file.
func randomFile(t *testing.T, dir string, size int64) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "random-")
	require.NoError(t, err)
	_, err = io.CopyN(f, rand.Reader, size)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func (h *pipelineHarness) uploadPair(t *testing.T, test model.DraftTest, size int64) {
	t.Helper()
	in, err := os.Open(randomFile(t, h.work, size))
	require.NoError(t, err)
	defer in.Close()
	answer, err := os.Open(randomFile(t, h.work, size))
	require.NoError(t, err)
	defer answer.Close()
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	_, err = h.uploads.Upload(h.ctx, h.draft.ProblemID, current.Revision, uuid.NewString(), model.UploadChange{Part: "pair", Test: &test}, in, answer)
	require.NoError(t, err)
}

func (h *pipelineHarness) enqueueAndClaim(t *testing.T) model.ProblemBuild {
	t.Helper()
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	_, err = h.repo.EnqueueBuild(h.ctx, mutationKey(h.org, "build", current.Revision), current.ProblemID, current.Revision, h.options)
	require.NoError(t, err)
	build, err := h.repo.ClaimBuild(h.ctx, h.options.Lease)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, build.ID)
	return build
}

const acceptEverythingChecker = "int main(){return 0;}\n"

// Heap use is bounded by buffers and concurrency, not by the total size of the tests: the
// build transfers hundreds of megabytes of incompressible data through a small heap.
func TestPipelineLargeIncompressibleTestsKeepHeapBounded(t *testing.T) {
	const perFile, tests = 24 << 20, 3
	h := newPipelineHarness(t, 64<<20, 2<<30)
	checker, subtask := uuid.New(), uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Files = map[string]string{"checkers/check.cpp": acceptEverythingChecker}
		d.Components = []model.DraftComponent{{ID: checker, Role: "checker", Path: "checkers/check.cpp", Language: "cpp"}}
		d.CheckerID = &checker
		d.Subtasks = []model.DraftSubtask{{ID: subtask, Ordinal: 1, Name: "All", Policy: "each"}}
	})
	for i := 1; i <= tests; i++ {
		h.uploadPair(t, model.DraftTest{
			ID: uuid.New(), Ordinal: i, SubtaskID: subtask, Points: 100,
			Input:  model.DraftInput{Mode: "manual"},
			Answer: model.DraftAnswer{Mode: "provided"},
		}, perFile)
	}
	// Execute runs without the worker's heartbeat, so the lease must outlive the whole build.
	h.options.Lease = 10 * time.Minute
	build := h.enqueueAndClaim(t)

	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	var peak atomic.Uint64
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		var m runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peak.Load() {
					peak.Store(m.HeapAlloc)
				}
			}
		}
	}()
	packageID, err := h.pipeline().Execute(h.ctx, build)
	close(stop)
	<-sampled
	require.NoError(t, err)

	var size int64
	var digest, key string
	require.NoError(t, h.pool.QueryRow(h.ctx, `SELECT o.size,o.sha256,o.key FROM problem_packages p JOIN stored_objects o ON o.id=p.object_id WHERE p.id=$1`, packageID).Scan(&size, &digest, &key))
	total := int64(2 * perFile * tests)
	require.Greater(t, size, total, "the archive holds every incompressible test body")
	requireStoredObject(t, h.ctx, h.services.store, "objects", key, size, digest)
	growth := int64(peak.Load()) - int64(baseline.HeapAlloc)
	t.Logf("tests=%d MiB archive=%d MiB heap growth=%d MiB", total>>20, size>>20, growth>>20)
	require.Less(t, growth, int64(48<<20), "heap must not grow with the total size of the tests (%d MiB of data)", total>>20)
	entries, err := os.ReadDir(h.work)
	require.NoError(t, err)
	for _, e := range entries {
		require.False(t, strings.HasPrefix(e.Name(), "build-"), "build directory %s must be removed", e.Name())
	}
}

// generatorBuild saves a draft whose only test is produced by the given generator script.
func (h *pipelineHarness) generatorBuild(t *testing.T, script string) {
	t.Helper()
	checker, generator, solution, subtask := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Files = map[string]string{"checkers/check.cpp": acceptEverythingChecker, "generators/gen.py": script, "solutions/main.py": "print(1)\n"}
		d.Components = []model.DraftComponent{
			{ID: checker, Role: "checker", Path: "checkers/check.cpp", Language: "cpp"},
			{ID: generator, Role: "generator", Path: "generators/gen.py", Language: "python"},
			{ID: solution, Role: "solution", Path: "solutions/main.py", Language: "python"},
		}
		d.CheckerID, d.MainSolutionID = &checker, &solution
		d.Subtasks = []model.DraftSubtask{{ID: subtask, Ordinal: 1, Name: "All", Policy: "each"}}
		seed := int64(1)
		d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: subtask, Points: 1,
			Input:  model.DraftInput{Mode: "generator", ComponentID: &generator, Args: []string{}, Seed: &seed},
			Answer: model.DraftAnswer{Mode: "generated", ComponentID: &solution}}}
	})
}

func waitBuild(t *testing.T, h *pipelineHarness, id uuid.UUID, states ...string) model.ProblemBuild {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		b, err := h.repo.GetBuild(h.ctx, id)
		require.NoError(t, err)
		for _, state := range states {
			if b.State == state {
				return b
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("build %s did not reach %v", id, states)
	return model.ProblemBuild{}
}

func (h *pipelineHarness) runWorker(t *testing.T) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	worker := &builder.Worker{Pipeline: h.pipeline(), Parallelism: 1, Lease: h.options.Lease, Poll: 100 * time.Millisecond}
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(30 * time.Second):
			t.Error("worker did not stop")
		}
	})
	return cancel
}

// A generator that exceeds the output budget fails the build as an author error with a
// bounded diagnostic, and nothing is published.
func TestPipelineOutputOverflowFailsBuildWithBoundedDiagnostic(t *testing.T) {
	h := newPipelineHarness(t, 1<<20, 256<<20)
	h.generatorBuild(t, "import sys\nsys.stdout.write('x' * (8 << 20))\n")
	_, err := h.repo.EnqueueBuild(h.ctx, mutationKey(h.org, "build", "overflow"), h.draft.ProblemID, mustRevision(t, h), h.options)
	require.NoError(t, err)
	h.runWorker(t)
	build := waitBuild(t, h, mustBuildID(t, h), "failed", "succeeded")
	require.Equal(t, "failed", build.State)
	var diagnostic struct {
		Stage, Kind, Message string
		Test                 int
		Truncated            bool
	}
	t.Logf("diagnostic: %s", build.Diagnostic)
	require.NoError(t, json.Unmarshal(build.Diagnostic, &diagnostic))
	require.Equal(t, "generate", diagnostic.Stage)
	require.Equal(t, "OLE", diagnostic.Kind)
	require.Equal(t, 1, diagnostic.Test)
	require.LessOrEqual(t, len(diagnostic.Message), 8192)
	require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, build.ID))
	require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_build_objects WHERE build_id=$1 AND role='output'`, build.ID))
}

// Cancelling a running build frees the single worker slot: a generator that would sleep for
// minutes is abandoned within a few heartbeats, and the next build completes.
func TestPipelineCancelFreesWorkerForNextBuild(t *testing.T) {
	h := newPipelineHarness(t, 1<<20, 256<<20)
	h.options.Limits.Auxiliary.Wall = 10 * time.Minute
	h.options.Limits.Auxiliary.CPU = 10 * time.Minute
	h.generatorBuild(t, "import time\ntime.sleep(300)\n")
	_, err := h.repo.EnqueueBuild(h.ctx, mutationKey(h.org, "build", "slow"), h.draft.ProblemID, mustRevision(t, h), h.options)
	require.NoError(t, err)
	slow := mustBuildID(t, h)
	h.runWorker(t)
	waitBuild(t, h, slow, "running")
	time.Sleep(2 * time.Second) // let the generator start inside the sandbox

	_, err = h.repo.CancelBuild(h.ctx, mutationKey(h.org, "cancel", slow), slow)
	require.NoError(t, err)

	h.generatorBuild(t, "print(1)\n")
	_, err = h.repo.EnqueueBuild(h.ctx, mutationKey(h.org, "build", "fast"), h.draft.ProblemID, mustRevision(t, h), h.options)
	require.NoError(t, err)
	builds, err := h.repo.ListBuilds(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	require.Len(t, builds, 2)
	fast := builds[0].ID
	if fast == slow {
		fast = builds[1].ID
	}
	started := time.Now()
	next := waitBuild(t, h, fast, "succeeded", "failed")
	require.Equal(t, "succeeded", next.State, "diagnostic: %s", next.Diagnostic)
	require.Less(t, time.Since(started), 90*time.Second, "the worker must not wait for the cancelled generator")

	cancelled, err := h.repo.GetBuild(h.ctx, slow)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelled.State)
	require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, slow))
	require.Equal(t, 1, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, fast))
	var left []string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		left, _ = filepath.Glob(filepath.Join(h.work, "build-*"))
		if len(left) == 0 {
			break
		}
	}
	require.Empty(t, left, "finished and cancelled builds leave no working directories")
}

func mustRevision(t *testing.T, h *pipelineHarness) int64 {
	t.Helper()
	d, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	return d.Revision
}

// mustBuildID returns the newest build of the problem.
func mustBuildID(t *testing.T, h *pipelineHarness) uuid.UUID {
	t.Helper()
	builds, err := h.repo.ListBuilds(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	require.NotEmpty(t, builds)
	return builds[0].ID
}

// The disk budget is enforced while materializing the snapshot, before any sandbox work.
func TestPipelineDiskQuotaRejectsOversizedSnapshot(t *testing.T) {
	h := newPipelineHarness(t, 16<<20, 4<<20)
	checker, subtask := uuid.New(), uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Files = map[string]string{"checkers/check.cpp": acceptEverythingChecker}
		d.Components = []model.DraftComponent{{ID: checker, Role: "checker", Path: "checkers/check.cpp", Language: "cpp"}}
		d.CheckerID = &checker
		d.Subtasks = []model.DraftSubtask{{ID: subtask, Ordinal: 1, Name: "All", Policy: "each"}}
	})
	h.uploadPair(t, model.DraftTest{ID: uuid.New(), Ordinal: 1, SubtaskID: subtask, Points: 1, Input: model.DraftInput{Mode: "manual"}, Answer: model.DraftAnswer{Mode: "provided"}}, 3<<20)
	build := h.enqueueAndClaim(t)
	_, err := h.pipeline().Execute(h.ctx, build)
	require.Error(t, err)
	var failure *builder.Failure
	require.True(t, errors.As(err, &failure), "quota errors are reported as a build failure: %v", err)
	require.Equal(t, "quota", failure.Kind, "a deterministic budget overrun is an author failure, not a retried infrastructure error")
	require.Contains(t, strings.ToLower(failure.Message), "quota")
	matches, err := filepath.Glob(filepath.Join(h.work, "build-*"))
	require.NoError(t, err)
	require.Empty(t, matches, "the working directory is removed")
}
