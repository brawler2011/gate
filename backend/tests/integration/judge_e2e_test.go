//go:build integration
// +build integration

package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/diskcache"
	"github.com/brawler2011/gate/backend/pkg/outputs"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	seaweedS3Port    = "8333/tcp"
	goJudgeHTTPPort  = "5050/tcp"
	e2eSandboxTimout = 90 * time.Second
)

func setupE2EEnvironment(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	return context.Background(), newTestDB(t)
}

// judgeNow schedules and judges a submission in place of a worker.
func judgeNow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, judge *judging.JudgeUseCase, id uuid.UUID) {
	t.Helper()
	jobs := pgstore.NewJudgeJobs(pool)
	_, err := jobs.ScheduleJob(ctx, id)
	require.NoError(t, err)
	job, ok, err := jobs.ClaimNext(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, id, job.SubmissionID)
	require.NoError(t, judge.JudgeSubmission(ctx, job))
}

func TestJudgeFullFlowE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	ctx, dbPool := setupE2EEnvironment(t)
	store, bucket := newSeaweedBackedS3Storage(t)
	client, files := newGoJudge(t)
	imageCtx, cancel := context.WithTimeout(ctx, e2eSandboxTimout)
	image, err := sandbox.ReadImage(imageCtx, client)
	cancel()
	require.NoError(t, err)
	source, err := sandbox.LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	require.Equal(t, source.Langs, image.Config.Langs)
	cfg := image.Config

	orgsRepo := pgstore.NewOrganizationsRepo(dbPool)
	usersRepo := pgstore.NewUsersRepo(dbPool)
	contestsRepo := pgstore.NewContestsRepo(dbPool)
	submissionsRepo := pgstore.NewSubmissionsRepo(dbPool)
	drafts := pgstore.NewProblemDraftsRepo(dbPool, problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Groups: 10})

	userID := uuid.New()
	email := "test@example.com"
	require.NoError(t, usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{Id: userID, Username: "testuser", Email: &email, PasswordHash: "hashed_password", Role: accessmodel.UserRoleUser}))
	orgID := uuid.New()
	require.NoError(t, dbfixtures.InsertOrganization(ctx, dbPool, orgID, "test-org", "Test Org", userID))
	org, err := orgsRepo.GetOrganizationByID(ctx, orgID)
	require.NoError(t, err)
	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser})

	problemID := uuid.New()
	require.NoError(t, dbfixtures.InsertProblem(ctx, dbPool, dbfixtures.ProblemSpec{ID: problemID, OrganizationID: org.ID, Owner: userID, Visibility: "public", Title: "Sum of Two Numbers", ShortName: "sum-problem"}))
	d, err := drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)
	generator, solution, group := "gen", "main", uuid.New()
	d.Checker = &problemsmodel.DraftChecker{DraftJury: problemsmodel.DraftJury{Protocol: sandbox.ProtocolTestlib}, DraftProgram: problemsmodel.DraftProgram{Language: "cpp17", Source: "#include <fstream>\nint main(int n,char**v){\n#ifdef REJECT_ALL\nreturn 1;\n#endif\nlong long a,b;std::ifstream out(v[2]),ans(v[3]);return !(out>>a) || !(ans>>b) || a!=b;}"}}
	d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1], sys.argv[2])\n"}}}
	d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: solution, DraftProgram: problemsmodel.DraftProgram{Language: "cpp17", Source: "#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<'\\n';}"}}, Verdicts: []string{}}}
	d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Print a+b."}}
	d.MainSolution = &solution
	d.Testset = problemsmodel.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []problemsmodel.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
	test := func(ordinal int, weight points.Value, a, b string) problemsmodel.DraftTest {
		return problemsmodel.DraftTest{ID: uuid.New(), Ordinal: ordinal, GroupID: group, Points: weight,
			Input:  problemsmodel.DraftInput{Mode: "generator", Generator: &generator, Args: []string{a, b}},
			Answer: problemsmodel.DraftAnswer{Mode: "generated", Solution: &solution}}
	}
	d.Tests = []problemsmodel.DraftTest{test(1, 40*points.Unit, "1", "2"), test(2, 60*points.Unit, "3", "2")}
	_, err = drafts.SaveDraft(ctx, d)
	require.NoError(t, err)
	d, err = drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)

	limits := dbfixtures.BuildLimits(t)
	_, err = drafts.EnqueueBuild(ctx, problemID, d.Revision, problemsmodel.BuildOptions{Languages: cfg.Langs, Limits: limits, MaxAttempts: 3, Duration: 2 * time.Minute, Lease: time.Minute})
	require.NoError(t, err)
	build, err := drafts.ClaimBuild(ctx, 5*time.Minute)
	require.NoError(t, err)
	work := diskTempDir(t)
	pipeline := builder.Pipeline{Repo: drafts, Store: store, Bucket: bucket, TempDir: work, Client: client, Typeset: dbfixtures.Typesetter(t), Files: files, Languages: cfg, Programs: sandbox.NewProgramCache(diskCache(t), client, files, sandbox.Image{Config: cfg}), Parallelism: judgeParallelism(t), Calibration: calibration.Uncalibrated, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	packageID, err := pipeline.Execute(ctx, build)
	require.NoError(t, err)

	// The checker cache is keyed by the judge image digest.
	cacheDir := diskTempDir(t)
	cache, err := diskcache.Open(cacheDir, 1<<30)
	require.NoError(t, err)
	packages, workDir := judgeCache(t)
	reads := &countingStorage{Storage: store, reads: map[string]int{}}
	newJudge := func(image sandbox.Image) *judging.JudgeUseCase {
		executor := &judging.PackageJudge{Client: client, Files: files, Languages: image.Config, Programs: sandbox.NewProgramCache(cache, client, files, image), Limits: limits, WorkDir: workDir, Calibration: calibration.Uncalibrated, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		return judging.NewJudgeUseCase(pgstore.NewJudgeJobs(dbPool), pgstore.NewJudgeTransactions(dbPool), submissionsRepo,
			&judging.PackageLoader{Store: reads, Bucket: bucket, Cache: packages, Packages: drafts, Limits: limits},
			executor, pgstore.NewNotifier(dbPool))
	}
	judgeUC := newJudge(image)
	cached := func(t *testing.T) []string {
		t.Helper()
		entries, err := os.ReadDir(cacheDir)
		require.NoError(t, err)
		names := []string{}
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				names = append(names, e.Name())
			}
		}
		return names
	}

	contestID := uuid.New()
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	require.NoError(t, dbfixtures.CreateContest(ctx, dbPool, contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Title: "E2E Contest", Login: "e2e-contest", Description: "E2E", Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &start, EndTime: &end}, contestOwner(t, ctx, dbPool)))
	require.NoError(t, contestsRepo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: problemID, PackageId: packageID}))
	require.NoError(t, contestsRepo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: userID, Role: accessmodel.ContestParticipant}))

	judge := func(t *testing.T, problem uuid.UUID, language string, source string) submissionsmodel.Submission {
		t.Helper()
		id, err := submissionsRepo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{Kind: submissionsmodel.KindContest, Solution: source, ProblemId: problem, ContestId: contestID, UserId: userID, Language: language})
		require.NoError(t, err)
		judgeNow(t, ctx, dbPool, judgeUC, id)
		sub, err := submissionsRepo.GetSubmission(ctx, id)
		require.NoError(t, err)
		return sub
	}

	t.Run("CorrectSolutionScoresEveryWeight", func(t *testing.T) {
		sub := judge(t, problemID, "python3", "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.Accepted, sub.State)
		assert.Equal(t, 100*points.Unit, sub.Score)
	})
	t.Run("WrongSolutionIsRejected", func(t *testing.T) {
		sub := judge(t, problemID, "python3", "print(42)")
		assert.Equal(t, submissionsmodel.GotWA, sub.State)
		assert.Equal(t, points.Value(0), sub.Score)
		require.NotNil(t, sub.FailedTest)
		assert.EqualValues(t, 1, *sub.FailedTest)
	})
	t.Run("CheckerCompiledOncePerImage", func(t *testing.T) {
		programs := cached(t)
		require.Len(t, programs, 1, "the first submissions compiled the checker into the cache")
		entry := filepath.Join(cacheDir, programs[0], "entry")
		before, err := os.Stat(entry)
		require.NoError(t, err)
		sub := judge(t, problemID, "python3", "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.Accepted, sub.State)
		after, err := os.Stat(entry)
		require.NoError(t, err)
		assert.True(t, os.SameFile(before, after), "a cached checker is not compiled again")
		assert.Equal(t, programs, cached(t))
	})
	t.Run("ChangedJudgeImageCompilesItsOwnChecker", func(t *testing.T) {
		// A different judge image recompiles the checker instead of reusing cached binaries.
		langs := maps.Clone(image.Config.Langs)
		cpp := langs["cpp17"]
		cpp.Compile = strings.Replace(cpp.Compile, "g++ ", "g++ -DREJECT_ALL ", 1)
		langs["cpp17"] = cpp
		changed := sandbox.Image{Identity: image.Identity, Config: &sandbox.Config{Langs: langs}}
		judgeUC = newJudge(changed)
		sub := judge(t, problemID, "python3", "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.GotWA, sub.State, "the checker compiled with the changed configuration rejects everything")
		assert.Len(t, cached(t), 2, "each image keeps its own compiled checker")
		judgeUC = newJudge(image)
		sub = judge(t, problemID, "python3", "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.Accepted, sub.State)
	})
	t.Run("PackageDownloadedOnce", func(t *testing.T) {
		descriptor, err := drafts.GetPackageDescriptor(ctx, packageID)
		require.NoError(t, err)
		assert.Equal(t, 1, reads.count(descriptor.Object.Key), "later jobs judge from the cached package")
	})
	t.Run("UnreadablePackageIsInternalError", func(t *testing.T) {
		// A Package whose archive object was never uploaded cannot be loaded.
		brokenProblem := uuid.New()
		require.NoError(t, dbfixtures.InsertProblem(ctx, dbPool, dbfixtures.ProblemSpec{ID: brokenProblem, OrganizationID: org.ID, Owner: userID, Visibility: "public", Title: "Broken", ShortName: "broken-problem"}))
		brokenPackage, err := dbfixtures.InsertPackage(ctx, dbPool, brokenProblem, org.ID, time.Time{})
		require.NoError(t, err)
		require.NoError(t, contestsRepo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: brokenProblem, PackageId: brokenPackage}))
		sub := judge(t, brokenProblem, "python3", "print(1)")
		assert.Equal(t, submissionsmodel.GotIE, sub.State)
	})
}

type countingStorage struct {
	storage.Storage
	mu    sync.Mutex
	reads map[string]int
}

func (s *countingStorage) DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	s.reads[key]++
	s.mu.Unlock()
	return s.Storage.DownloadFile(ctx, bucket, key, ifNoneMatch)
}

func (s *countingStorage) count(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads[key]
}

type judgeHarness struct {
	ctx         context.Context
	pool        *pgxpool.Pool
	drafts      *pgstore.ProblemDraftsRepo
	contests    *pgstore.ContestsRepo
	submissions *pgstore.SubmissionsRepo
	judge       *judging.JudgeUseCase
	pipeline    builder.Pipeline
	limits      problemsmodel.BuildLimits
	orgID       uuid.UUID
	userID      uuid.UUID
	contestID   uuid.UUID
	store       *storage.S3Storage
	bucket      string
}

func newJudgeHarness(t *testing.T) *judgeHarness {
	t.Helper()
	ctx, dbPool := setupE2EEnvironment(t)
	store, bucket := newSeaweedBackedS3Storage(t)
	client, files := newGoJudge(t)
	imageCtx, cancel := context.WithTimeout(ctx, e2eSandboxTimout)
	image, err := sandbox.ReadImage(imageCtx, client)
	cancel()
	require.NoError(t, err)
	source, err := sandbox.LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	require.Equal(t, source.Langs, image.Config.Langs)
	cfg := image.Config

	h := &judgeHarness{pool: dbPool, userID: uuid.New(), contestID: uuid.New(), store: store, bucket: bucket}
	orgsRepo := pgstore.NewOrganizationsRepo(dbPool)
	usersRepo := pgstore.NewUsersRepo(dbPool)
	h.contests = pgstore.NewContestsRepo(dbPool)
	h.submissions = pgstore.NewSubmissionsRepo(dbPool)
	h.drafts = pgstore.NewProblemDraftsRepo(dbPool, problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Groups: 10})

	email := "test@example.com"
	require.NoError(t, usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{Id: h.userID, Username: "testuser", Email: &email, PasswordHash: "hashed_password", Role: accessmodel.UserRoleUser}))
	h.orgID = uuid.New()
	require.NoError(t, dbfixtures.InsertOrganization(ctx, dbPool, h.orgID, "test-org", "Test Org", h.userID))
	org, err := orgsRepo.GetOrganizationByID(ctx, h.orgID)
	require.NoError(t, err)
	h.ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: h.userID, Role: accessmodel.UserRoleUser})

	h.limits = dbfixtures.BuildLimits(t)
	programs := sandbox.NewProgramCache(diskCache(t), client, files, image)
	h.pipeline = builder.Pipeline{Repo: h.drafts, Store: store, Bucket: bucket, TempDir: diskTempDir(t), Client: client, Typeset: dbfixtures.Typesetter(t), Files: files, Languages: cfg, Programs: programs, Parallelism: judgeParallelism(t), Calibration: calibration.Uncalibrated, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	packages, workDir := judgeCache(t)
	executor := &judging.PackageJudge{Client: client, Files: files, Languages: cfg, Programs: programs, Limits: h.limits, WorkDir: workDir, Calibration: calibration.Uncalibrated, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h.judge = judging.NewJudgeUseCase(pgstore.NewJudgeJobs(dbPool), pgstore.NewJudgeTransactions(dbPool), h.submissions,
		&judging.PackageLoader{Store: store, Bucket: bucket, Cache: packages, Packages: h.drafts, Limits: h.limits},
		executor, pgstore.NewNotifier(dbPool))

	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	require.NoError(t, dbfixtures.CreateContest(h.ctx, h.pool, contestsmodel.CreateContestParams{ID: h.contestID, OrganizationID: org.ID, Title: "E2E Contest", Login: "e2e-contest", Description: "E2E", Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &start, EndTime: &end}, contestOwner(t, h.ctx, h.pool)))
	require.NoError(t, h.contests.CreateContestMember(h.ctx, &contestsmodel.CreateContestMemberParams{ContestId: h.contestID, UserId: h.userID, Role: accessmodel.ContestParticipant}))
	return h
}

func (h *judgeHarness) build(t *testing.T, shortName string, edit func(*problemsmodel.Draft)) (problemID, packageID uuid.UUID, err error) {
	t.Helper()
	ctx := h.ctx
	problemID = uuid.New()
	require.NoError(t, dbfixtures.InsertProblem(ctx, h.pool, dbfixtures.ProblemSpec{ID: problemID, OrganizationID: h.orgID, Owner: h.userID, Visibility: "public", Title: shortName, ShortName: shortName}))
	d, err := h.drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)
	edit(&d)
	_, err = h.drafts.SaveDraft(ctx, d)
	require.NoError(t, err)
	d, err = h.drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)
	_, err = h.drafts.EnqueueBuild(ctx, problemID, d.Revision, problemsmodel.BuildOptions{Languages: h.pipeline.Languages.Langs, Limits: h.limits, MaxAttempts: 3, Duration: 2 * time.Minute, Lease: time.Minute})
	require.NoError(t, err)
	build, err := h.drafts.ClaimBuild(ctx, 5*time.Minute)
	require.NoError(t, err)
	packageID, err = h.pipeline.Execute(ctx, build)
	return problemID, packageID, err
}

func (h *judgeHarness) publish(t *testing.T, shortName string, edit func(*problemsmodel.Draft)) uuid.UUID {
	t.Helper()
	problemID, packageID, err := h.build(t, shortName, edit)
	require.NoError(t, err)
	require.NoError(t, h.contests.CreateContestProblem(h.ctx, contestsmodel.ContestProblemCreation{ContestId: h.contestID, ProblemId: problemID, PackageId: packageID}))
	return problemID
}

func (h *judgeHarness) submit(t *testing.T, problem uuid.UUID, language string, source string) submissionsmodel.Submission {
	t.Helper()
	id, err := h.submissions.CreateSubmission(h.ctx, &submissionsmodel.SubmissionCreation{Kind: submissionsmodel.KindContest, Solution: source, ProblemId: problem, ContestId: h.contestID, UserId: h.userID, Language: language})
	require.NoError(t, err)
	judgeNow(t, h.ctx, h.pool, h.judge, id)
	sub, err := h.submissions.GetSubmission(h.ctx, id)
	require.NoError(t, err)
	return sub
}

// Interactive verdicts may come from the interactor or from a checker comparing recorded dialogue.
func TestInteractiveJudgingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	h := newJudgeHarness(t)
	interactor := func(status string) string {
		return "import sys\na = int(open(sys.argv[1]).read())\nprint(a, flush=True)\nreply = input().strip()\nopen(sys.argv[2], 'w').write(reply + '\\n')\nsys.exit(" + status + ")\n"
	}
	draft := func(interactorSource, checkerSource string) func(*problemsmodel.Draft) {
		return func(d *problemsmodel.Draft) {
			generator, solution, group := "gen", "main", uuid.New()
			d.Type = "interactive"
			testlib := problemsmodel.DraftJury{Protocol: sandbox.ProtocolTestlib}
			d.Checker = &problemsmodel.DraftChecker{DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: checkerSource}, DraftJury: testlib}
			d.Interactor = &problemsmodel.DraftInteractor{DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: interactorSource}, DraftJury: testlib}
			d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1])\n"}}}
			d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: solution, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "print(2 * int(input()), flush=True)\n"}}, Verdicts: []string{}}}
			d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Reply with twice the number."}}
			d.MainSolution = &solution
			d.Testset = problemsmodel.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []problemsmodel.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
			d.Tests = []problemsmodel.DraftTest{{ID: uuid.New(), Ordinal: 1, GroupID: group, Points: 100 * points.Unit,
				Input:  problemsmodel.DraftInput{Mode: "generator", Generator: &generator, Args: []string{"5"}},
				Answer: problemsmodel.DraftAnswer{Mode: "interactive"}}}
		}
	}
	const (
		acceptAll = "import sys\nsys.exit(0)\n"
		correct   = "print(2 * int(input()), flush=True)"
		wrong     = "print(int(input()) + 1, flush=True)"
	)
	cases := map[string]uuid.UUID{
		// Judging compares the dialogue against the answer generated during the build.
		"InteractorDecides": h.publish(t, "interactor-decides", draft(interactor("0 if len(sys.argv) < 4 or reply == open(sys.argv[3]).read().strip() else 1"), acceptAll)),
		"CheckerDecides": h.publish(t, "checker-decides", draft(interactor("0"),
			"import sys\nout, ans = open(sys.argv[2]).read().split(), open(sys.argv[3]).read().split()\nsys.exit(0 if out == ans else 1)\n")),
	}
	for name, problemID := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, submissionsmodel.Accepted, h.submit(t, problemID, "python3", correct).State)
			assert.Equal(t, submissionsmodel.GotWA, h.submit(t, problemID, "python3", wrong).State)
		})
	}
	t.Run("InteractorGivesPoints", func(t *testing.T) {
		// testlib quitp: exit 7 with the test's points, here 30 of 100 for an answer one off.
		scorer := "import sys\na = int(open(sys.argv[1]).read())\nprint(a, flush=True)\nreply = int(input())\nopen(sys.argv[2], 'w').write(str(reply) + '\\n')\n" +
			"if reply == a + 1:\n    sys.stderr.write('points 30\\n')\n    sys.exit(7)\nsys.exit(0 if reply == 2 * a else 1)\n"
		problemID := h.publish(t, "interactor-points", func(d *problemsmodel.Draft) {
			draft(scorer, acceptAll)(d)
			d.Grading = "points"
		})
		full := h.submit(t, problemID, "python3", correct)
		assert.Equal(t, submissionsmodel.Accepted, full.State)
		assert.Equal(t, 100*points.Unit, full.Score)
		partial := h.submit(t, problemID, "python3", wrong)
		assert.Equal(t, submissionsmodel.Partial, partial.State)
		assert.Equal(t, 30*points.Unit, partial.Score)
		require.NotNil(t, partial.TestDetails)
		assert.Equal(t, "POINTS", partial.TestDetails.Tests[0].Verdict)
		zero := h.submit(t, problemID, "python3", "print(0, flush=True)")
		assert.Equal(t, submissionsmodel.GotWA, zero.State)
		assert.Equal(t, points.Value(0), zero.Score)
	})
	t.Run("CheckerRejectingMainSolutionFailsBuild", func(t *testing.T) {
		_, _, err := h.build(t, "checker-rejects", draft(interactor("0"), "import sys\nsys.exit(1)\n"))
		var failure *builder.Failure
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, "check", failure.Stage)
	})
}

// A communication problem runs both copies of the solution against one manager, which relays copy 0's message to copy 1.
func TestCommunicationJudgingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	h := newJudgeHarness(t)
	const (
		manager = "import sys\na = open(sys.argv[1]).read().strip()\n" +
			"def ask(i, message):\n    with open(6 + 2 * i, 'w') as w:\n        w.write(message + '\\n')\n    return open(5 + 2 * i).readline().strip()\n" +
			"reply = ask(1, ask(0, a))\nopen(sys.argv[2], 'w').write(reply + '\\n')\nsys.exit(0 if reply == a else 1)\n"
		encode  = "import sys\nx = int(input())\nif sys.argv[1] == '0':\n    print(2 * x, flush=True)\nelse:\n"
		correct = encode + "    print(x // 2, flush=True)\n"
		wrong   = encode + "    print(x, flush=True)\n"
		slow    = encode + "    while True:\n        pass\n"
	)
	problemID := h.publish(t, "communication", func(d *problemsmodel.Draft) {
		generator, solution, group := "gen", "main", uuid.New()
		d.Type = "communication"
		d.Processes = 2
		d.Interactor = &problemsmodel.DraftInteractor{DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: manager}, DraftJury: problemsmodel.DraftJury{Protocol: sandbox.ProtocolTestlib}}
		d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1])\n"}}}
		d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: solution, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: correct}}, Verdicts: []string{}}}
		d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Pass the number from the first copy to the second."}}
		d.MainSolution = &solution
		d.Testset = problemsmodel.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []problemsmodel.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
		d.Tests = []problemsmodel.DraftTest{{ID: uuid.New(), Ordinal: 1, GroupID: group, Points: 100 * points.Unit,
			Input:  problemsmodel.DraftInput{Mode: "generator", Generator: &generator, Args: []string{"21"}},
			Answer: problemsmodel.DraftAnswer{Mode: "interactive"}}}
	})
	assert.Equal(t, submissionsmodel.Accepted, h.submit(t, problemID, "python3", correct).State)
	assert.Equal(t, submissionsmodel.GotWA, h.submit(t, problemID, "python3", wrong).State)
	assert.Equal(t, submissionsmodel.GotTL, h.submit(t, problemID, "python3", slow).State)
}

// Standard checkers build with the system testlib.h without a draft copy.
func TestStandardCheckerE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	h := newJudgeHarness(t)
	draft := func(standard, solution string) func(*problemsmodel.Draft) {
		return func(d *problemsmodel.Draft) {
			generator, main, group := "gen", "main", uuid.New()
			d.Checker = &problemsmodel.DraftChecker{DraftProgram: problemsmodel.DraftProgram{Language: "cpp17"}, DraftJury: problemsmodel.DraftJury{Protocol: sandbox.ProtocolTestlib}, Standard: standard}
			d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1], sys.argv[2])\n"}}}
			d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: main, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: solution}}, Verdicts: []string{}}}
			d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Compute."}}
			d.MainSolution = &main
			d.Testset = problemsmodel.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []problemsmodel.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
			for i, args := range [][]string{{"1", "3"}, {"2", "7"}} {
				d.Tests = append(d.Tests, problemsmodel.DraftTest{ID: uuid.New(), Ordinal: i + 1, GroupID: group, Points: 50 * points.Unit,
					Input:  problemsmodel.DraftInput{Mode: "generator", Generator: &generator, Args: args},
					Answer: problemsmodel.DraftAnswer{Mode: "generated", Solution: &main}})
			}
		}
	}
	const read = "a, b = map(int, input().split())\n"
	cases := []struct {
		standard, solution, accepted, wrong string
	}{
		{"wcmp", read + "print(a, b)", read + "print(' ', a, '\\n\\n', b)", read + "print(b, a)"},
		{"rcmp6", read + "print('%.10f' % (a / b))", read + "print('%.7f' % (a / b))", read + "print('%.4f' % (a / b + 1e-4))"},
	}
	for _, c := range cases {
		problemID := h.publish(t, c.standard, draft(c.standard, c.solution))
		t.Run(c.standard, func(t *testing.T) {
			assert.Equal(t, submissionsmodel.Accepted, h.submit(t, problemID, "python3", c.accepted).State)
			assert.Equal(t, submissionsmodel.GotWA, h.submit(t, problemID, "python3", c.wrong).State)
		})
	}
}

// Output-only judging checks each submitted output; a test without one is WA, and the others still score.
func TestOutputOnlyJudgingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	h := newJudgeHarness(t)
	problemID := h.publish(t, "output-only", func(d *problemsmodel.Draft) {
		generator, main, group := "gen", "main", uuid.New()
		d.Type, d.Grading = "output-only", "points"
		d.Checker = &problemsmodel.DraftChecker{DraftProgram: problemsmodel.DraftProgram{Language: "cpp17"}, DraftJury: problemsmodel.DraftJury{Protocol: sandbox.ProtocolTestlib}, Standard: "wcmp"}
		d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1])\n"}}}
		d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: main, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "print(2 * int(input()))\n"}}, Verdicts: []string{}}}
		d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Double each number."}}
		d.MainSolution = &main
		d.Testset = problemsmodel.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []problemsmodel.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
		for i, arg := range []string{"1", "2", "3"} {
			d.Tests = append(d.Tests, problemsmodel.DraftTest{ID: uuid.New(), Ordinal: i + 1, GroupID: group, Points: 10 * points.Unit,
				Input:  problemsmodel.DraftInput{Mode: "generator", Generator: &generator, Args: []string{arg}},
				Answer: problemsmodel.DraftAnswer{Mode: "generated", Solution: &main}})
		}
		max := 30 * points.Unit
		d.MaxScore = &max
	})

	link, err := h.contests.GetContestProblem(h.ctx, contestsmodel.ContestProblemGet{ContestId: h.contestID, ProblemId: problemID})
	require.NoError(t, err)
	content := &problems.PackageContent{Repo: h.drafts, Store: h.store, Bucket: h.bucket}
	inputs, err := content.OpenFiles(h.ctx, link.PackageID)
	require.NoError(t, err)
	data, err := io.ReadAll(inputs)
	require.NoError(t, inputs.Close())
	require.NoError(t, err)
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	names := []string{}
	for _, f := range z.File {
		names = append(names, f.Name)
	}
	assert.Equal(t, []string{"1.in", "2.in", "3.in"}, names)

	archive, sha, err := outputs.Pack([]outputs.File{{Name: "01.out", Data: strings.NewReader("2\n")}, {Name: "02.out", Data: strings.NewReader("4\n")}})
	require.NoError(t, err)
	require.NoError(t, h.store.UploadFile(h.ctx, h.bucket, outputs.Key(sha), bytes.NewReader(archive), "application/zip"))
	id, err := h.submissions.CreateSubmission(h.ctx, &submissionsmodel.SubmissionCreation{Kind: submissionsmodel.KindContest, Outputs: &sha, ProblemId: problemID, ContestId: h.contestID, UserId: h.userID, Language: submissionsmodel.OutputsLanguage})
	require.NoError(t, err)
	judgeNow(t, h.ctx, h.pool, h.judge, id)
	sub, err := h.submissions.GetSubmission(h.ctx, id)
	require.NoError(t, err)
	assert.Equal(t, submissionsmodel.Partial, sub.State)
	assert.Equal(t, 20*points.Unit, sub.Score)
	require.NotNil(t, sub.TestDetails)
	verdicts := []string{}
	for _, test := range sub.TestDetails.Tests {
		verdicts = append(verdicts, test.Verdict)
	}
	assert.Equal(t, []string{"OK", "OK", "WA"}, verdicts)
}

// A grader problem links each solution with the jury's grader; a solution with its own main does not link.
func TestGraderJudgingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	h := newJudgeHarness(t)
	const header = "long long sum(long long a, long long b);\n"
	solution := func(body string) string { return "#include \"sum.h\"\nlong long sum(long long a, long long b) { return " + body + "; }\n" }
	problemID := h.publish(t, "grader", func(d *problemsmodel.Draft) {
		generator, main, group := "gen", "main", uuid.New()
		d.Checker = &problemsmodel.DraftChecker{DraftProgram: problemsmodel.DraftProgram{Language: "cpp17"}, DraftJury: problemsmodel.DraftJury{Protocol: sandbox.ProtocolTestlib}, Standard: "wcmp"}
		d.Graders = []problemsmodel.DraftFile{{Name: "sum.h", Content: header},
			{Name: "grader.cpp", Content: "#include <cstdio>\n#include \"sum.h\"\nint main() { long long a, b; scanf(\"%lld %lld\", &a, &b); printf(\"%lld\\n\", sum(a, b)); }\n"}}
		d.Attachments = []problemsmodel.DraftFile{{Name: "sum.h", Content: header}}
		d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1], sys.argv[2])\n"}}}
		d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: main, DraftProgram: problemsmodel.DraftProgram{Language: "cpp17", Source: solution("a + b")}}, Verdicts: []string{}}}
		d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Implement sum."}}
		d.MainSolution = &main
		d.Testset = problemsmodel.DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete", Groups: []problemsmodel.DraftGroup{{ID: group, Name: "All", Aggregate: "sum", Feedback: "complete"}}}
		for i, args := range [][]string{{"1", "2"}, {"3", "-4"}} {
			d.Tests = append(d.Tests, problemsmodel.DraftTest{ID: uuid.New(), Ordinal: i + 1, GroupID: group, Points: 50 * points.Unit,
				Input:  problemsmodel.DraftInput{Mode: "generator", Generator: &generator, Args: args},
				Answer: problemsmodel.DraftAnswer{Mode: "generated", Solution: &main}})
		}
	})
	link, err := h.contests.GetContestProblem(h.ctx, contestsmodel.ContestProblemGet{ContestId: h.contestID, ProblemId: problemID})
	require.NoError(t, err)
	assert.Contains(t, link.Languages, "cpp17")
	assert.NotContains(t, link.Languages, "c")
	assert.NotContains(t, link.Languages, "python3")
	assert.True(t, link.Files)

	assert.Equal(t, submissionsmodel.Accepted, h.submit(t, problemID, "cpp17", solution("a + b")).State)
	assert.Equal(t, submissionsmodel.GotWA, h.submit(t, problemID, "cpp17", solution("a - b")).State)
	own := h.submit(t, problemID, "cpp17", solution("a + b")+"int main() {}\n")
	assert.Equal(t, submissionsmodel.GotCE, own.State)
	require.NotNil(t, own.TestDetails)
	require.NotNil(t, own.TestDetails.CompilerOutput)
	assert.Contains(t, *own.TestDetails.CompilerOutput, "multiple definition of `main'")
}

// contestOwner keeps the judged user a plain participant.
func contestOwner(t *testing.T, ctx context.Context, db dbfixtures.Execer) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, dbfixtures.InsertUser(ctx, db, id))
	return id
}
