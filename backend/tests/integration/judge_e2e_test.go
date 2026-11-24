//go:build integration
// +build integration

package integration

import (
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
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/diskcache"
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
	drafts := pgstore.NewProblemDraftsRepo(dbPool, problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10})

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
	d.Checker = &problemsmodel.DraftChecker{DraftProgram: problemsmodel.DraftProgram{Language: "cpp17", Source: "#include <fstream>\nint main(int n,char**v){\n#ifdef REJECT_ALL\nreturn 1;\n#endif\nlong long a,b;std::ifstream out(v[2]),ans(v[3]);return !(out>>a) || !(ans>>b) || a!=b;}"}}
	d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1], sys.argv[2])\n"}}}
	d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: solution, DraftProgram: problemsmodel.DraftProgram{Language: "cpp17", Source: "#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<'\\n';}"}}, Verdicts: []string{}}}
	d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Print a+b."}}
	d.MainSolution = &solution
	d.Subtasks = []problemsmodel.DraftSubtask{{ID: group, Ordinal: 1, Name: "All", Policy: "complete", Feedback: "complete"}}
	test := func(ordinal int, weight points.Value, a, b string) problemsmodel.DraftTest {
		return problemsmodel.DraftTest{ID: uuid.New(), Ordinal: ordinal, SubtaskID: group, Points: weight,
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

	h := &judgeHarness{pool: dbPool, userID: uuid.New(), contestID: uuid.New()}
	orgsRepo := pgstore.NewOrganizationsRepo(dbPool)
	usersRepo := pgstore.NewUsersRepo(dbPool)
	h.contests = pgstore.NewContestsRepo(dbPool)
	h.submissions = pgstore.NewSubmissionsRepo(dbPool)
	h.drafts = pgstore.NewProblemDraftsRepo(dbPool, problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10})

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
			d.Checker = &problemsmodel.DraftChecker{DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: checkerSource}}
			d.Interactor = &problemsmodel.DraftProgram{Language: "python3", Source: interactorSource}
			d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1])\n"}}}
			d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: solution, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "print(2 * int(input()), flush=True)\n"}}, Verdicts: []string{}}}
			d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Reply with twice the number."}}
			d.MainSolution = &solution
			d.Subtasks = []problemsmodel.DraftSubtask{{ID: group, Ordinal: 1, Name: "All", Policy: "complete", Feedback: "complete"}}
			d.Tests = []problemsmodel.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: group, Points: 100 * points.Unit,
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
	t.Run("CheckerRejectingMainSolutionFailsBuild", func(t *testing.T) {
		_, _, err := h.build(t, "checker-rejects", draft(interactor("0"), "import sys\nsys.exit(1)\n"))
		var failure *builder.Failure
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, "check", failure.Stage)
	})
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
			d.Checker = &problemsmodel.DraftChecker{DraftProgram: problemsmodel.DraftProgram{Language: "cpp17"}, Standard: standard}
			d.Generators = []problemsmodel.DraftNamedProgram{{Name: generator, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: "import sys\nprint(sys.argv[1], sys.argv[2])\n"}}}
			d.Solutions = []problemsmodel.DraftSolution{{DraftNamedProgram: problemsmodel.DraftNamedProgram{Name: main, DraftProgram: problemsmodel.DraftProgram{Language: "python3", Source: solution}}, Verdicts: []string{}}}
			d.Statements = []problemsmodel.DraftStatement{{Language: "en", Legend: "Compute."}}
			d.MainSolution = &main
			d.Subtasks = []problemsmodel.DraftSubtask{{ID: group, Ordinal: 1, Name: "All", Policy: "complete", Feedback: "complete"}}
			for i, args := range [][]string{{"1", "3"}, {"2", "7"}} {
				d.Tests = append(d.Tests, problemsmodel.DraftTest{ID: uuid.New(), Ordinal: i + 1, SubtaskID: group, Points: 50 * points.Unit,
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


// contestOwner keeps the judged user a plain participant.
func contestOwner(t *testing.T, ctx context.Context, db dbfixtures.Execer) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, dbfixtures.InsertUser(ctx, db, id))
	return id
}
