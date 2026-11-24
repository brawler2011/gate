//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/judging"
	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/hostcheck"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

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
	programs *sandbox.ProgramCache
}

func newPipelineHarness(t *testing.T, objectBytes, diskBytes int64) *pipelineHarness {
	t.Helper()
	ctx, pool, repo, org, d := draftFixture(t)
	services := startPipelineServices(t)
	work := dbfixtures.DiskTempDir(t)
	// The tests exercise these quotas.
	limits := dbfixtures.BuildLimits(t)
	limits.DiskBytes, limits.FileCount = bytesize.Size(diskBytes), 100
	limits.ObjectBytes, limits.TestOutputBytes, limits.Auxiliary.Output = bytesize.Size(objectBytes), bytesize.Size(objectBytes), bytesize.Size(objectBytes)
	options := postgres.ProblemBuildOptions{
		Languages: services.langs,
		Limits:    limits, MaxAttempts: 1, Duration: 5 * time.Minute, Lease: 3 * time.Second,
	}
	uploads := &problems.DraftUseCase{
		DraftQuotas: model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Groups: 10},
		Repo:        repo, Problems: fixedProblem{org}, Access: postgres.NewAccess(pool), Store: services.store, Bucket: "objects",
		TempDir: work, UploadTTL: 5 * time.Minute, BuildOptions: options,
		Languages: &sandbox.Config{Langs: services.langs},
	}
	return &pipelineHarness{ctx: ctx,
		pool: pool, repo: repo, org: org, draft: d, services: services, work: work, limits: limits, options: options, uploads: uploads,
		programs: sandbox.NewProgramCache(dbfixtures.DiskCache(t), services.client, services.files, sandbox.Image{Config: &sandbox.Config{Langs: services.langs}})}
}

func (h *pipelineHarness) pipeline() *builder.Pipeline {
	return &builder.Pipeline{Repo: h.repo, Store: h.services.store, Bucket: "objects", TempDir: h.work, Client: h.services.client, Typeset: h.services.typeset,
		Files: h.services.files, Languages: &sandbox.Config{Langs: h.options.Languages}, Programs: h.programs, Parallelism: 2, Calibration: calibration.Uncalibrated, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func (h *pipelineHarness) save(t *testing.T, change func(*model.Draft)) {
	t.Helper()
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	change(&current)
	_, err = h.repo.SaveDraft(h.ctx, current)
	require.NoError(t, err)
}

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
	_, err = h.uploads.Upload(h.ctx, h.draft.ProblemID, current.Revision, model.UploadChange{Test: &test}, map[string]io.Reader{"input": in, "answer": answer})
	require.NoError(t, err)
}

func (h *pipelineHarness) enqueueAndClaim(t *testing.T) model.ProblemBuild {
	t.Helper()
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	_, err = h.repo.EnqueueBuild(h.ctx, current.ProblemID, current.Revision, h.options)
	require.NoError(t, err)
	build, err := h.repo.ClaimBuild(h.ctx, h.options.Lease)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, build.ID)
	return build
}

const acceptEverythingChecker = "int main(){return 0;}\n"

// Build heap use stays bounded as test data grows.
func TestPipelineLargeIncompressibleTestsKeepHeapBounded(t *testing.T) {
	// Run sequentially because the heap measurement covers the entire process.
	const perFile, tests = 24 << 20, 3
	h := newPipelineHarness(t, 64<<20, 2<<30)
	group := uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Checker = checker("cpp17", acceptEverythingChecker)
		d.Testset = model.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []model.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
	})
	for i := 1; i <= tests; i++ {
		h.uploadPair(t, model.DraftTest{
			ID: uuid.New(), Ordinal: i, GroupID: group, Points: 100,
			Input:  model.DraftInput{Mode: "manual"},
			Answer: model.DraftAnswer{Mode: "provided"},
		}, perFile)
	}
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

// Builds use the running judge catalog; missing configurations are system errors.
func TestPipelineBuildsWithRunningImageLanguages(t *testing.T) {
	t.Parallel()
	h := newPipelineHarness(t, 1<<20, 64<<20)
	group := uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Checker = checker("cpp17", "#ifndef NEW_IMAGE\n#error compiled with the enqueue-time languages\n#endif\n"+acceptEverythingChecker)
		d.Testset = model.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []model.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
	})
	h.uploadPair(t, model.DraftTest{ID: uuid.New(), Ordinal: 1, GroupID: group, Points: 100, Input: model.DraftInput{Mode: "manual"}, Answer: model.DraftAnswer{Mode: "provided"}}, 16)

	without := maps.Clone(h.options.Languages)
	delete(without, "cpp17")
	pipeline := h.pipeline()
	pipeline.Languages = &sandbox.Config{Langs: without}
	pipeline.Programs = sandbox.NewProgramCache(dbfixtures.DiskCache(t), h.services.client, h.services.files, sandbox.Image{Config: pipeline.Languages})
	_, err := pipeline.Execute(h.ctx, h.enqueueAndClaim(t))
	var failure *builder.Failure
	require.ErrorAs(t, err, &failure)
	require.Equal(t, "compile", failure.Stage)
	require.Equal(t, "configuration", failure.Kind)
	require.Contains(t, failure.Message, "checkers/checker")

	changed := maps.Clone(h.options.Languages)
	cpp := changed["cpp17"]
	cpp.Compile = strings.Replace(cpp.Compile, "g++ ", "g++ -DNEW_IMAGE ", 1)
	changed["cpp17"] = cpp
	pipeline.Languages = &sandbox.Config{Langs: changed}
	pipeline.Programs = sandbox.NewProgramCache(dbfixtures.DiskCache(t), h.services.client, h.services.files, sandbox.Image{Config: pipeline.Languages})
	_, err = pipeline.Execute(h.ctx, h.enqueueAndClaim(t))
	require.NoError(t, err)
}

func (h *pipelineHarness) generatorBuild(t *testing.T, script string) {
	t.Helper()
	generator, solution, group := "gen", "main", uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Checker = checker("cpp17", acceptEverythingChecker)
		d.Generators = []model.DraftNamedProgram{named(generator, "python3", script)}
		d.Solutions = []model.DraftSolution{solutionProgram(solution, "python3", "print(1)\n")}
		d.MainSolution = &solution
		d.Testset = model.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []model.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
		d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, GroupID: group, Points: 1,
			Input:  model.DraftInput{Mode: "generator", Generator: &generator, Args: []string{}},
			Answer: model.DraftAnswer{Mode: "generated", Solution: &solution}}}
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
	worker := &builder.Worker{Pipeline: h.pipeline(), Parallelism: 1, Lease: h.options.Lease, Poll: 100 * time.Millisecond, Gate: hostcheck.Open{}}
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

// Excess generator output fails publication with a bounded author diagnostic.
func TestPipelineOutputOverflowFailsBuildWithBoundedDiagnostic(t *testing.T) {
	t.Parallel()
	h := newPipelineHarness(t, 1<<20, 256<<20)
	h.generatorBuild(t, "import sys\nsys.stdout.write('x' * (8 << 20))\n")
	_, err := h.repo.EnqueueBuild(h.ctx, h.draft.ProblemID, mustRevision(t, h), h.options)
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
	require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM stored_objects o WHERE o.purpose='package' AND NOT EXISTS(SELECT 1 FROM problem_packages p WHERE p.object_id=o.id)`))
}

// Cancelling a build frees the worker slot for the next build.
func TestPipelineCancelFreesWorkerForNextBuild(t *testing.T) {
	t.Parallel()
	h := newPipelineHarness(t, 1<<20, 256<<20)
	h.options.Limits.Auxiliary = h.options.Limits.WithCPU(h.options.Limits.Auxiliary, 10*time.Minute)
	h.generatorBuild(t, "import time\ntime.sleep(300)\n")
	_, err := h.repo.EnqueueBuild(h.ctx, h.draft.ProblemID, mustRevision(t, h), h.options)
	require.NoError(t, err)
	slow := mustBuildID(t, h)
	h.runWorker(t)
	waitBuild(t, h, slow, "running")
	time.Sleep(2 * time.Second) // Cancellation must interrupt a running generator.

	err = h.repo.CancelBuild(h.ctx, slow)
	require.NoError(t, err)

	h.generatorBuild(t, "print(1)\n")
	_, err = h.repo.EnqueueBuild(h.ctx, h.draft.ProblemID, mustRevision(t, h), h.options)
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

func mustBuildID(t *testing.T, h *pipelineHarness) uuid.UUID {
	t.Helper()
	builds, err := h.repo.ListBuilds(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	require.NotEmpty(t, builds)
	return builds[0].ID
}

// The disk budget is enforced while materializing the snapshot, before any sandbox work.
// Tests sharing an input share one package file, so the package unpacks within the budget the build fit in.
func TestPipelineBuildsTestsSharingAnInput(t *testing.T) {
	t.Parallel()
	h := newPipelineHarness(t, 4<<20, 16<<20)
	group := uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Checker = checker("cpp17", acceptEverythingChecker)
		d.Testset = model.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []model.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
	})
	first := model.DraftTest{ID: uuid.New(), Ordinal: 1, GroupID: group, Points: 1, Input: model.DraftInput{Mode: "manual"}, Answer: model.DraftAnswer{Mode: "provided"}}
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	_, err = h.uploads.Upload(h.ctx, h.draft.ProblemID, current.Revision, model.UploadChange{Test: &first},
		map[string]io.Reader{"input": bytes.NewReader(make([]byte, 3<<20)), "answer": strings.NewReader("0\n")})
	require.NoError(t, err)
	h.save(t, func(d *model.Draft) {
		for i := 2; i <= 6; i++ {
			test := d.Tests[0]
			test.ID, test.Ordinal = uuid.New(), i
			d.Tests = append(d.Tests, test)
		}
	})
	packageID, err := h.pipeline().Execute(h.ctx, h.enqueueAndClaim(t))
	require.NoError(t, err)
	loader := &judging.PackageLoader{Store: h.services.store, Bucket: "objects", Cache: dbfixtures.DiskCache(t), Packages: h.repo, Limits: h.limits}
	pkg, err := loader.LoadPackage(h.ctx, packageID)
	require.NoError(t, err, "judges unpack the package")
	pkg.Release()
}

func TestPipelineDiskQuotaRejectsOversizedSnapshot(t *testing.T) {
	t.Parallel()
	h := newPipelineHarness(t, 16<<20, 4<<20)
	group := uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Checker = checker("cpp17", acceptEverythingChecker)
		d.Testset = model.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []model.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
	})
	h.uploadPair(t, model.DraftTest{ID: uuid.New(), Ordinal: 1, GroupID: group, Points: 1, Input: model.DraftInput{Mode: "manual"}, Answer: model.DraftAnswer{Mode: "provided"}}, 3<<20)
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
