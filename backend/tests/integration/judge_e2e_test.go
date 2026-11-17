//go:build integration
// +build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	judgeworker "github.com/brawler2011/gate/backend/internal/messaging/judge"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	seaweedS3Port    = "8333/tcp"
	goJudgeHTTPPort  = "5050/tcp"
	goJudgeGRPCPort  = "5051/tcp"
	e2eSandboxTimout = 90 * time.Second
)

func setupE2EEnvironment(t *testing.T) (context.Context, *pgxpool.Pool, jetstream.JetStream) {
	t.Helper()
	ctx := context.Background()
	dbPool := newTestDB(t)
	js, _ := newNATSJetStream(t)
	require.NoError(t, pkg.EnsureSubmissionsStream(ctx, js))
	return ctx, dbPool, js
}

func TestJudgeFullFlowE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	ctx, dbPool, js := setupE2EEnvironment(t)
	store, bucket := newSeaweedBackedS3Storage(t)
	client, files := newGoJudge(t)
	// The languages come from the judge image, as in production.
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
	drafts := pgstore.NewProblemDraftsRepo(dbPool, problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10}, time.Minute)

	userID := uuid.New()
	email := "test@example.com"
	require.NoError(t, usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{Id: userID, Username: "testuser", Email: &email, PasswordHash: "hashed_password", Role: accessmodel.UserRoleUser}))
	org, err := orgsRepo.CreateOrganization(ctx, &organizationsmodel.CreateOrganizationInput{Login: "test-org", Name: "Test Org"})
	require.NoError(t, err)
	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser})

	// Draft: generated inputs (1 2 and 3 2), answers produced by the main solution,
	// explicit weights 40.00 and 60.00 in one "complete" group.
	problemID := uuid.New()
	require.NoError(t, dbfixtures.InsertProblem(ctx, dbPool, dbfixtures.ProblemSpec{ID: problemID, OrganizationID: org.ID, OwnerID: &userID, Visibility: "public", Title: "Sum of Two Numbers", ShortName: "sum-problem"}))
	d, err := drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)
	checker, generator, solution, group := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	d.Files = map[string]string{
		"checkers/check.cpp": "#include <fstream>\nint main(int n,char**v){\n#ifdef REJECT_ALL\nreturn 1;\n#endif\nlong long a,b;std::ifstream out(v[2]),ans(v[3]);return !(out>>a) || !(ans>>b) || a!=b;}",
		"generators/gen.py":  "import sys\nprint(sys.argv[1], sys.argv[2])\n",
		"solutions/main.cpp": "#include <iostream>\nint main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<'\\n';}",
		"statements/en.md":   "Print a+b.",
	}
	d.Components = []problemsmodel.DraftComponent{
		{ID: checker, Role: "checker", Path: "checkers/check.cpp"},
		{ID: generator, Role: "generator", Path: "generators/gen.py"},
		{ID: solution, Role: "solution", Path: "solutions/main.cpp"},
	}
	d.MainSolutionID = &solution
	d.Subtasks = []problemsmodel.DraftSubtask{{ID: group, Ordinal: 1, Name: "All", Policy: "complete"}}
	seed := int64(1)
	test := func(ordinal int, weight points.Value, a, b string) problemsmodel.DraftTest {
		return problemsmodel.DraftTest{ID: uuid.New(), Ordinal: ordinal, SubtaskID: group, Points: weight,
			Input:  problemsmodel.DraftInput{Mode: "generator", ComponentID: &generator, Args: []string{a, b}, Seed: &seed},
			Answer: problemsmodel.DraftAnswer{Mode: "generated", ComponentID: &solution}}
	}
	d.Tests = []problemsmodel.DraftTest{test(1, 40*points.Unit, "1", "2"), test(2, 60*points.Unit, "3", "2")}
	saved, err := drafts.SaveDraft(ctx, mutationKey(org.ID, "edit", d), d)
	require.NoError(t, err)
	require.NotEmpty(t, saved.Body)
	d, err = drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)

	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 10 * time.Second, Memory: 256 << 20, Processes: 50, Output: 32 << 20, Diagnostic: 4096}
	compile := aux
	compile.CPU, compile.Wall, compile.Memory, compile.Processes = 10*time.Second, 20*time.Second, 512<<20, 150
	limits := problemsmodel.BuildLimits{DiskBytes: 256 << 20, FileCount: 100, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: compile, Auxiliary: aux}
	_, err = drafts.EnqueueBuild(ctx, mutationKey(org.ID, "build", d.Revision), problemID, d.Revision, problemsmodel.BuildOptions{Languages: cfg.Langs, Limits: limits, MaxAttempts: 3, Duration: 2 * time.Minute, Lease: time.Minute})
	require.NoError(t, err)
	build, err := drafts.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	work := diskTempDir(t)
	pipeline := builder.Pipeline{Repo: drafts, Store: store, Bucket: bucket, TempDir: work, Client: client, Files: files, Languages: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	packageID, err := pipeline.Execute(ctx, build)
	require.NoError(t, err)

	// A judge compiles the package's checker with its own image; digest names that image.
	cache := diskTempDir(t)
	newJudge := func(image sandbox.Image) *judging.JudgeUseCase {
		executor := &judging.PackageJudge{Client: client, Files: files, Languages: image.Config, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		executor.Programs = &judging.ProgramCache{Root: cache, Compiler: image.Compiler(), Compile: executor.CompileProgram}
		return judging.NewJudgeUseCase(pgstore.NewJudgeJobs(dbPool), pgstore.NewJudgeTransactions(dbPool), submissionsRepo,
			&judging.PackageLoader{Store: store, Bucket: bucket, TempDir: diskTempDir(t), Packages: drafts, Limits: limits},
			executor, judgeworker.NewEventPublisher(js))
	}
	judgeUC := newJudge(image)
	checkerEntry := func(image sandbox.Image) string {
		return filepath.Join(cache, packageID.String(), image.Compiler(), "checker", "entry")
	}

	contestID := uuid.New()
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	require.NoError(t, contestsRepo.CreateContest(ctx, &contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, OwnerID: &userID, Title: "E2E Contest", Login: "e2e-contest", Description: "E2E", Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &start, EndTime: &end, Settings: map[string]interface{}{}}))
	require.NoError(t, contestsRepo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: problemID, PackageId: packageID}))
	require.NoError(t, contestsRepo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: userID, Role: contestsmodel.ContestRoleParticipant}))

	judge := func(t *testing.T, problem uuid.UUID, language submissionsmodel.LanguageName, source string) submissionsmodel.Submission {
		t.Helper()
		id, err := submissionsRepo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{Solution: source, ProblemId: problem, ContestId: contestID, UserId: userID, Language: language})
		require.NoError(t, err)
		job, err := pgstore.NewJudgeJobs(dbPool).ScheduleJob(ctx, id)
		require.NoError(t, err)
		require.NoError(t, judgeUC.JudgeSubmission(ctx, job.Ref()))
		sub, err := submissionsRepo.GetSubmission(ctx, id)
		require.NoError(t, err)
		return sub
	}

	t.Run("CorrectSolutionScoresEveryWeight", func(t *testing.T) {
		sub := judge(t, problemID, submissionsmodel.Python, "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.Accepted, sub.State)
		assert.Equal(t, 100*points.Unit, sub.Score)
	})
	t.Run("WrongSolutionIsRejected", func(t *testing.T) {
		sub := judge(t, problemID, submissionsmodel.Python, "print(42)")
		assert.Equal(t, submissionsmodel.GotWA, sub.State)
		assert.Equal(t, points.Value(0), sub.Score)
		require.NotNil(t, sub.FailedTest)
		assert.EqualValues(t, 1, *sub.FailedTest)
	})
	t.Run("CheckerCompiledOncePerImage", func(t *testing.T) {
		before, err := os.Stat(checkerEntry(image))
		require.NoError(t, err, "the first submissions compiled the checker into the cache")
		sub := judge(t, problemID, submissionsmodel.Python, "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.Accepted, sub.State)
		after, err := os.Stat(checkerEntry(image))
		require.NoError(t, err)
		assert.True(t, os.SameFile(before, after), "a cached checker is not compiled again")
	})
	t.Run("ChangedJudgeImageCompilesItsOwnChecker", func(t *testing.T) {
		// The package carries no binaries: a worker whose image differs, here only in its
		// language configuration, compiles the checker itself instead of reusing the cached one.
		langs := maps.Clone(image.Config.Langs)
		cpp := langs["cpp"]
		cpp.Compile = strings.Replace(cpp.Compile, "g++ ", "g++ -DREJECT_ALL ", 1)
		langs["cpp"] = cpp
		changed := sandbox.Image{Identity: image.Identity, Config: &sandbox.Config{Langs: langs}}
		judgeUC = newJudge(changed)
		sub := judge(t, problemID, submissionsmodel.Python, "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.GotWA, sub.State, "the checker compiled with the changed configuration rejects everything")
		_, err := os.Stat(checkerEntry(changed))
		require.NoError(t, err)
		judgeUC = newJudge(image)
		sub = judge(t, problemID, submissionsmodel.Python, "print(sum(map(int, input().split())))")
		assert.Equal(t, submissionsmodel.Accepted, sub.State)
	})
	t.Run("UnreadablePackageIsInternalError", func(t *testing.T) {
		// A Package whose archive object was never uploaded cannot be loaded.
		brokenProblem := uuid.New()
		require.NoError(t, dbfixtures.InsertProblem(ctx, dbPool, dbfixtures.ProblemSpec{ID: brokenProblem, OrganizationID: org.ID, OwnerID: &userID, Visibility: "public", Title: "Broken", ShortName: "broken-problem"}))
		brokenPackage, err := dbfixtures.InsertPackage(ctx, dbPool, brokenProblem, org.ID, time.Time{})
		require.NoError(t, err)
		require.NoError(t, contestsRepo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: brokenProblem, PackageId: brokenPackage}))
		sub := judge(t, brokenProblem, submissionsmodel.Python, "print(1)")
		assert.Equal(t, submissionsmodel.GotIE, sub.State)
	})
}

// judgeHarness builds drafts on the real judge and judges submissions to them
// in one contest.
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
	ctx, dbPool, js := setupE2EEnvironment(t)
	store, bucket := newSeaweedBackedS3Storage(t)
	client, files := newGoJudge(t)
	// The languages come from the judge image, as in production.
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
	h.drafts = pgstore.NewProblemDraftsRepo(dbPool, problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 10, Subtasks: 10}, time.Minute)

	email := "test@example.com"
	require.NoError(t, usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{Id: h.userID, Username: "testuser", Email: &email, PasswordHash: "hashed_password", Role: accessmodel.UserRoleUser}))
	org, err := orgsRepo.CreateOrganization(ctx, &organizationsmodel.CreateOrganizationInput{Login: "test-org", Name: "Test Org"})
	require.NoError(t, err)
	h.orgID = org.ID
	h.ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: h.userID, Role: accessmodel.UserRoleUser})

	aux := sandbox.FileRunLimits{CPU: 5 * time.Second, Wall: 10 * time.Second, Memory: 256 << 20, Processes: 50, Output: 32 << 20, Diagnostic: 4096}
	compile := aux
	compile.CPU, compile.Wall, compile.Memory, compile.Processes = 10*time.Second, 20*time.Second, 512<<20, 150
	h.limits = problemsmodel.BuildLimits{DiskBytes: 256 << 20, FileCount: 100, ObjectBytes: 32 << 20, TestOutputBytes: 32 << 20, Compile: compile, Auxiliary: aux}
	h.pipeline = builder.Pipeline{Repo: h.drafts, Store: store, Bucket: bucket, TempDir: diskTempDir(t), Client: client, Files: files, Languages: cfg, UploadTTL: time.Minute, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	executor := &judging.PackageJudge{Client: client, Files: files, Languages: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	executor.Programs = &judging.ProgramCache{Root: diskTempDir(t), Compiler: image.Compiler(), Compile: executor.CompileProgram}
	h.judge = judging.NewJudgeUseCase(pgstore.NewJudgeJobs(dbPool), pgstore.NewJudgeTransactions(dbPool), h.submissions,
		&judging.PackageLoader{Store: store, Bucket: bucket, TempDir: diskTempDir(t), Packages: h.drafts, Limits: h.limits},
		executor, judgeworker.NewEventPublisher(js))

	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	require.NoError(t, h.contests.CreateContest(h.ctx, &contestsmodel.CreateContestParams{ID: h.contestID, OrganizationID: org.ID, OwnerID: &h.userID, Title: "E2E Contest", Login: "e2e-contest", Description: "E2E", Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &start, EndTime: &end, Settings: map[string]interface{}{}}))
	require.NoError(t, h.contests.CreateContestMember(h.ctx, &contestsmodel.CreateContestMemberParams{ContestId: h.contestID, UserId: h.userID, Role: contestsmodel.ContestRoleParticipant}))
	return h
}

// build saves the draft that edit fills in to a new problem and builds it.
func (h *judgeHarness) build(t *testing.T, shortName string, edit func(*problemsmodel.Draft)) (problemID, packageID uuid.UUID, err error) {
	t.Helper()
	ctx := h.ctx
	problemID = uuid.New()
	require.NoError(t, dbfixtures.InsertProblem(ctx, h.pool, dbfixtures.ProblemSpec{ID: problemID, OrganizationID: h.orgID, OwnerID: &h.userID, Visibility: "public", Title: shortName, ShortName: shortName}))
	d, err := h.drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)
	edit(&d)
	saved, err := h.drafts.SaveDraft(ctx, mutationKey(h.orgID, "edit", d), d)
	require.NoError(t, err)
	require.NotEmpty(t, saved.Body)
	d, err = h.drafts.GetDraft(ctx, problemID)
	require.NoError(t, err)
	_, err = h.drafts.EnqueueBuild(ctx, mutationKey(h.orgID, "build", d.Revision), problemID, d.Revision, problemsmodel.BuildOptions{Languages: h.pipeline.Languages.Langs, Limits: h.limits, MaxAttempts: 3, Duration: 2 * time.Minute, Lease: time.Minute})
	require.NoError(t, err)
	build, err := h.drafts.ClaimBuild(ctx, time.Minute)
	require.NoError(t, err)
	packageID, err = h.pipeline.Execute(ctx, build)
	return problemID, packageID, err
}

// publish builds the draft and adds its package to the contest.
func (h *judgeHarness) publish(t *testing.T, shortName string, edit func(*problemsmodel.Draft)) uuid.UUID {
	t.Helper()
	problemID, packageID, err := h.build(t, shortName, edit)
	require.NoError(t, err)
	require.NoError(t, h.contests.CreateContestProblem(h.ctx, contestsmodel.ContestProblemCreation{ContestId: h.contestID, ProblemId: problemID, PackageId: packageID}))
	return problemID
}

func (h *judgeHarness) submit(t *testing.T, problem uuid.UUID, language submissionsmodel.LanguageName, source string) submissionsmodel.Submission {
	t.Helper()
	id, err := h.submissions.CreateSubmission(h.ctx, &submissionsmodel.SubmissionCreation{Solution: source, ProblemId: problem, ContestId: h.contestID, UserId: h.userID, Language: language})
	require.NoError(t, err)
	job, err := pgstore.NewJudgeJobs(h.pool).ScheduleJob(h.ctx, id)
	require.NoError(t, err)
	require.NoError(t, h.judge.JudgeSubmission(h.ctx, job.Ref()))
	sub, err := h.submissions.GetSubmission(h.ctx, id)
	require.NoError(t, err)
	return sub
}

// An interactive problem may leave the verdict to its interactor or, as is
// common in Polygon, let the interactor record the dialogue and the checker
// compare it with the answer: the interactor's output on the main solution.
func TestInteractiveJudgingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	t.Parallel()
	h := newJudgeHarness(t)
	// The interactor sends a; the solution must reply 2a. The interactor writes
	// the reply to its output and exits with the given status expression.
	interactor := func(status string) string {
		return "import sys\na = int(open(sys.argv[1]).read())\nprint(a, flush=True)\nreply = input().strip()\nopen(sys.argv[2], 'w').write(reply + '\\n')\nsys.exit(" + status + ")\n"
	}
	draft := func(interactorSource, checkerSource string) func(*problemsmodel.Draft) {
		return func(d *problemsmodel.Draft) {
			checker, intr, generator, solution, group := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
			d.Type = "interactive"
			d.Files = map[string]string{
				"checkers/check.py":       checkerSource,
				"interactors/interact.py": interactorSource,
				"generators/gen.py":       "import sys\nprint(sys.argv[1])\n",
				"solutions/main.py":       "print(2 * int(input()), flush=True)\n",
				"statements/en.md":        "Reply with twice the number.",
			}
			d.Components = []problemsmodel.DraftComponent{
				{ID: checker, Role: "checker", Path: "checkers/check.py"},
				{ID: intr, Role: "interactor", Path: "interactors/interact.py"},
				{ID: generator, Role: "generator", Path: "generators/gen.py"},
				{ID: solution, Role: "solution", Path: "solutions/main.py"},
			}
			d.MainSolutionID = &solution
			d.Subtasks = []problemsmodel.DraftSubtask{{ID: group, Ordinal: 1, Name: "All", Policy: "complete"}}
			seed := int64(1)
			d.Tests = []problemsmodel.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: group, Points: 100 * points.Unit,
				Input:  problemsmodel.DraftInput{Mode: "generator", ComponentID: &generator, Args: []string{"5"}, Seed: &seed},
				Answer: problemsmodel.DraftAnswer{Mode: "interactive"}}}
		}
	}
	const (
		acceptAll = "import sys\nsys.exit(0)\n"
		correct   = "print(2 * int(input()), flush=True)"
		wrong     = "print(int(input()) + 1, flush=True)"
	)
	cases := map[string]uuid.UUID{
		// The build has no answer yet and trusts the main solution; judging
		// compares the reply with the answer the interactor receives.
		"InteractorDecides": h.publish(t, "interactor-decides", draft(interactor("0 if len(sys.argv) < 4 or reply == open(sys.argv[3]).read().strip() else 1"), acceptAll)),
		"CheckerDecides": h.publish(t, "checker-decides", draft(interactor("0"),
			"import sys\nout, ans = open(sys.argv[2]).read().split(), open(sys.argv[3]).read().split()\nsys.exit(0 if out == ans else 1)\n")),
	}
	for name, problemID := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, submissionsmodel.Accepted, h.submit(t, problemID, submissionsmodel.Python, correct).State)
			assert.Equal(t, submissionsmodel.GotWA, h.submit(t, problemID, submissionsmodel.Python, wrong).State)
		})
	}
	t.Run("CheckerRejectingMainSolutionFailsBuild", func(t *testing.T) {
		_, _, err := h.build(t, "checker-rejects", draft(interactor("0"), "import sys\nsys.exit(1)\n"))
		var failure *builder.Failure
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, "check", failure.Stage)
	})
}

func mutationKey(org uuid.UUID, operation string, body any) problemsmodel.MutationKey {
	data, _ := json.Marshal(body)
	sum := sha256.Sum256(data)
	return problemsmodel.MutationKey{OrganizationID: org, Operation: operation, Key: uuid.NewString(), Fingerprint: hex.EncodeToString(sum[:]), Token: uuid.New()}
}
