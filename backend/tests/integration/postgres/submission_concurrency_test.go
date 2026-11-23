//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/internal/judging"
	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
	"github.com/brawler2011/gate/backend/internal/submissions"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type concurrencyFixture struct {
	pool             *pgxpool.Pool
	ctx              context.Context
	contest, problem uuid.UUID
	users            [3]uuid.UUID
	start            time.Time
	loader           access.Loader
	blockers         blockerPool
}

func newConcurrencyFixture(t *testing.T) *concurrencyFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	f := &concurrencyFixture{pool: getTestPool(t), ctx: ctx, contest: uuid.New(), problem: uuid.New(), start: time.Now().Add(-time.Hour).Truncate(time.Second)}
	org := helperCreateOrg(t, ctx, f.pool, uuid.NewString(), "Concurrency")
	for i := range f.users {
		f.users[i] = uuid.New()
		role := "user"
		if i == 0 {
			role = "admin"
		}
		_, err := f.pool.Exec(ctx, `INSERT INTO users(id,username,password_hash,role) VALUES ($1,$2,'unused',$3)`, f.users[i], uuid.NewString(), role)
		require.NoError(t, err)
	}
	end := f.start.Add(3 * time.Hour)
	repo := postgres.NewContestsRepo(f.pool)
	require.NoError(t, dbfixtures.CreateContest(ctx, f.pool, contestsmodel.CreateContestParams{ID: f.contest, OrganizationID: org, Title: "Concurrent", Login: uuid.NewString(), Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &f.start, EndTime: &end}, f.users[0]))
	for _, user := range f.users[1:] {
		require.NoError(t, repo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: f.contest, UserId: user, Role: accessmodel.ContestParticipant}))
	}
	err := dbfixtures.InsertProblem(ctx, f.pool, dbfixtures.ProblemSpec{ID: f.problem, OrganizationID: org, Owner: f.users[0], Title: "Problem", ShortName: "a"})
	require.NoError(t, err)
	pack, err := dbfixtures.InsertPackage(ctx, f.pool, f.problem, org, time.Time{})
	require.NoError(t, err)
	require.NoError(t, repo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem, PackageId: pack}))
	f.loader = postgres.NewAccess(f.pool)
	f.blockers = newBlockerPool(t)
	return f
}

func (f *concurrencyFixture) userContext(i int) context.Context {
	role := accessmodel.UserRoleUser
	if i == 0 {
		role = accessmodel.UserRoleAdmin
	}
	return accessmodel.WithUser(f.ctx, accessmodel.User{Id: f.users[i], Role: role})
}
func (f *concurrencyFixture) submissionUC(tx submissions.SubmissionTransactions) *submissions.SubmissionsUseCase {
	return submissions.NewSubmissionsUseCase(postgres.NewSubmissionsRepo(f.pool), tx, f.loader)
}
func (f *concurrencyFixture) create(tx submissions.SubmissionTransactions, user int) (uuid.UUID, error) {
	return f.submissionUC(tx).CreateSubmission(f.userContext(user), &submissionsmodel.SubmissionCreation{ContestId: f.contest, ProblemId: f.problem, UserId: f.users[user], Solution: "print(1)", Language: "python3"})
}
func (f *concurrencyFixture) job(t *testing.T, user, minute int) judgingmodel.Job {
	t.Helper()
	id, err := f.create(postgres.NewSubmissionTransactions(f.pool), user)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, "UPDATE submissions SET created_at=$2 WHERE id=$1", id, f.start.Add(time.Duration(minute)*time.Minute))
	require.NoError(t, err)
	// Claim this submission's job directly: other tests' submissions may be pending too.
	job := judgingmodel.Job{SubmissionID: id, LeaseToken: uuid.New()}
	require.NoError(t, f.pool.QueryRow(f.ctx, `UPDATE judge_jobs SET state='processing',attempt=attempt+1,lease_token=$2,deadline_at=now()+interval '30 seconds'
 WHERE submission_id=$1 RETURNING id,revision,package_id,package_hash,attempt`, id, job.LeaseToken).Scan(&job.ID, &job.Revision, &job.PackageID, &job.PackageHash, &job.Attempt))
	return job
}

// Verdict transactions cover notices under contention.
func applyConcurrentVerdict(ctx context.Context, tx judging.JudgeTxStore, job judgingmodel.Job, state submissionsmodel.State) error {
	current, err := tx.CompleteJob(ctx, job)
	if err != nil || !current {
		return err
	}
	if _, err := tx.UpdateSubmission(ctx, job.SubmissionID, &submissionsmodel.SubmissionUpdate{State: state}); err != nil {
		return err
	}
	return tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCompleted, ID: job.SubmissionID})
}
func (f *concurrencyFixture) complete(job judgingmodel.Job, state submissionsmodel.State) error {
	return postgres.NewJudgeTransactions(f.pool).Within(f.ctx, func(tx judging.JudgeTxStore) error { return applyConcurrentVerdict(f.ctx, tx, job, state) })
}

type barrierSubmissionTransactions struct {
	submissions.SubmissionTransactions
	barrier func() error
}

func (tx barrierSubmissionTransactions) Within(ctx context.Context, fn func(submissions.SubmissionTxStore) error) error {
	return tx.SubmissionTransactions.Within(ctx, func(store submissions.SubmissionTxStore) error {
		if err := fn(store); err != nil {
			return err
		}
		return tx.barrier()
	})
}

func (f *concurrencyFixture) holdTuple(t *testing.T, blockers blockerPool, user int) (release func(commit bool) error) {
	t.Helper()
	return holdTransaction(t, func(ctx context.Context, barrier func() error) error {
		return postgres.NewSubmissionTransactions(blockers.Pool).Within(ctx, func(tx submissions.SubmissionTxStore) error {
			if err := tx.LockSubmissionTuple(ctx, f.contest, f.users[user], f.problem); err != nil {
				return err
			}
			return barrier()
		})
	})
}

func holdSubmission(t *testing.T, blockers blockerPool, id uuid.UUID) (release func(commit bool) error) {
	t.Helper()
	return holdTransaction(t, func(ctx context.Context, barrier func() error) error {
		return pgx.BeginFunc(ctx, blockers.Pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT 1 FROM submissions WHERE id=$1 FOR UPDATE", id); err != nil {
				return err
			}
			return barrier()
		})
	})
}

func beginConcurrencyTx(t *testing.T, f *concurrencyFixture) pgx.Tx {
	t.Helper()
	tx, err := f.blockers.Begin(f.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}
func awaitConcurrent(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("operation blocked behind an independent transaction")
	}
}
func asyncConcurrent(fn func() error) <-chan error {
	result := make(chan error, 1)
	go func() { result <- fn() }()
	return result
}

func TestIndependentSubmissionTransactions(t *testing.T) {
	t.Parallel()
	for _, held := range []string{"creation", "completion", "administrative edit"} {
		t.Run(held, func(t *testing.T) {
			f := newConcurrencyFixture(t)
			first, other := f.job(t, 1, 10), f.job(t, 2, 20)
			switch held {
			case "creation":
				holdTransaction(t, func(_ context.Context, barrier func() error) error {
					_, err := f.create(barrierSubmissionTransactions{postgres.NewSubmissionTransactions(f.blockers.Pool), barrier}, 1)
					return err
				})
			case "completion":
				holdTransaction(t, func(ctx context.Context, barrier func() error) error {
					return postgres.NewJudgeTransactions(f.blockers.Pool).Within(ctx, func(tx judging.JudgeTxStore) error {
						if err := applyConcurrentVerdict(ctx, tx, first, submissionsmodel.Accepted); err != nil {
							return err
						}
						return barrier()
					})
				})
			default:
				tx := beginConcurrencyTx(t, f)
				_, err := tx.Exec(f.ctx, "SELECT id FROM contests WHERE id=$1 FOR NO KEY UPDATE", f.contest)
				require.NoError(t, err)
			}
			awaitConcurrent(t, asyncConcurrent(func() error { _, err := f.create(postgres.NewSubmissionTransactions(f.pool), 2); return err }))
			awaitConcurrent(t, asyncConcurrent(func() error { return f.complete(other, submissionsmodel.Accepted) }))
			sub, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, other.SubmissionID)
			require.NoError(t, err)
			require.Equal(t, submissionsmodel.Accepted, sub.State)
			require.EqualValues(t, 2, sub.Revision)
		})
	}
}

func TestCompletionRacesAdministrativeMutation(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"rejudge", "block submission", "block problem"} {
		t.Run(mutation, func(t *testing.T) {
			f := newConcurrencyFixture(t)
			job := f.job(t, 1, 10)
			release := holdSubmission(t, f.blockers, job.SubmissionID)
			completion := asyncConcurrent(func() error { return f.complete(job, submissionsmodel.Accepted) })
			uc := f.submissionUC(postgres.NewSubmissionTransactions(f.pool))
			update := asyncConcurrent(func() error {
				switch mutation {
				case "rejudge":
					_, err := uc.RejudgeSubmissions(f.userContext(0), submissionsmodel.RejudgeFilter{ContestID: f.contest, SubmissionID: &job.SubmissionID})
					return err
				case "block submission":
					return uc.BlockSubmission(f.userContext(0), f.contest, job.SubmissionID, nil)
				default:
					return contests.NewContestsUseCase(postgres.NewContestsRepo(f.pool), nil, postgres.NewContestCommandTransactions(f.pool), f.loader).BlockProblemForUser(f.userContext(0), f.contest, f.users[1], f.problem, nil)
				}
			})
			f.blockers.waitBlocked(t, f.ctx, 2)
			require.NoError(t, release(true))
			awaitConcurrent(t, completion)
			awaitConcurrent(t, update)
			// A late result from the old revision has no effect regardless of race order.
			require.NoError(t, f.complete(job, submissionsmodel.Accepted))
			sub, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
			require.NoError(t, err)
			expected := submissionsmodel.Disqualified
			if mutation == "rejudge" {
				expected = submissionsmodel.Saved
			}
			require.Equal(t, expected, sub.State)
		})
	}
}

func TestCreationRacesProblemBlock(t *testing.T) {
	t.Parallel()
	f := newConcurrencyFixture(t)
	release := f.holdTuple(t, f.blockers, 1)
	var id uuid.UUID
	create := asyncConcurrent(func() error {
		var err error
		id, err = f.create(postgres.NewSubmissionTransactions(f.pool), 1)
		return err
	})
	block := asyncConcurrent(func() error {
		return contests.NewContestsUseCase(postgres.NewContestsRepo(f.pool), nil, postgres.NewContestCommandTransactions(f.pool), f.loader).BlockProblemForUser(f.userContext(0), f.contest, f.users[1], f.problem, nil)
	})
	f.blockers.waitBlocked(t, f.ctx, 2)
	require.NoError(t, release(true))
	awaitConcurrent(t, create)
	awaitConcurrent(t, block)
	sub, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, id)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Disqualified, sub.State)
}

func TestBulkRejudgeLockingAndRollback(t *testing.T) {
	t.Parallel()
	f := newConcurrencyFixture(t)
	jobs := []judgingmodel.Job{f.job(t, 1, 10), f.job(t, 2, 20)}
	// Hold both submissions to observe lock order; a command waiting on the first must not lock the second.
	blockers := map[int]blockerPool{1: newBlockerPool(t), 2: newBlockerPool(t)}
	releases := map[int]func(bool) error{}
	for user, pool := range blockers {
		releases[user] = holdSubmission(t, pool, jobs[user-1].SubmissionID)
	}
	uc := f.submissionUC(postgres.NewSubmissionTransactions(f.pool))
	results := []<-chan error{}
	for range 2 {
		results = append(results, asyncConcurrent(func() error {
			_, err := uc.RejudgeSubmissions(f.userContext(0), submissionsmodel.RejudgeFilter{ContestID: f.contest})
			return err
		}))
	}
	first := 0
	require.Eventually(t, func() bool {
		for user, pool := range blockers {
			if n, err := pool.blocked(f.ctx); err == nil && n >= 2 {
				first = user
				return true
			}
		}
		return false
	}, 3*time.Second, 5*time.Millisecond)
	// Bulk commands exclude candidates inserted after their candidate snapshot.
	newUser := 3 - first
	require.NoError(t, releases[newUser](true))
	var created uuid.UUID
	awaitConcurrent(t, asyncConcurrent(func() error {
		var err error
		created, err = f.create(postgres.NewSubmissionTransactions(f.pool), newUser)
		return err
	}))
	require.NoError(t, releases[first](true))
	for _, result := range results {
		awaitConcurrent(t, result)
	}
	// A serializable retry takes a fresh snapshot, so each bulk command rejudges the late submission at most once.
	late, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, created)
	require.NoError(t, err)
	require.LessOrEqual(t, late.Revision, int64(3))
	var lateJobs int64
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM judge_jobs WHERE submission_id=$1", created).Scan(&lateJobs))
	require.Equal(t, late.Revision, lateJobs)
	sentinel := errors.New("rollback bulk rejudge")
	err = postgres.NewSubmissionTransactions(f.pool).Within(f.ctx, func(tx submissions.SubmissionTxStore) error {
		changed, err := tx.ResetSubmissionsState(f.ctx, submissionsmodel.RejudgeFilter{ContestID: f.contest})
		if err != nil {
			return err
		}
		for _, change := range changed {
			if _, err := tx.ScheduleJob(f.ctx, change.ID); err != nil {
				return err
			}
			if err := tx.Notify(f.ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCreated, ID: change.ID}); err != nil {
				return err
			}
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	for _, job := range jobs {
		sub, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
		require.NoError(t, err)
		require.EqualValues(t, 3, sub.Revision)
		var count int
		require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM judge_jobs WHERE submission_id=$1", sub.ID).Scan(&count))
		require.Equal(t, 3, count)
	}
}

func TestConcurrentDraftLimit(t *testing.T) {
	t.Parallel()
	f := newConcurrencyFixture(t)
	_, err := f.pool.Exec(f.ctx, `INSERT INTO contest_drafts(contest_id,user_id,code) SELECT $1,$2,'draft' FROM generate_series(1,49)`, f.contest, f.users[1])
	require.NoError(t, err)
	release := holdTransaction(t, func(ctx context.Context, barrier func() error) error {
		return postgres.NewDraftTransactions(f.blockers.Pool).Within(ctx, func(tx contests.DraftTxStore) error {
			if err := tx.LockDraftOwner(ctx, f.contest, f.users[1]); err != nil {
				return err
			}
			return barrier()
		})
	})
	uc := contests.NewDraftsUseCase(postgres.NewDraftTransactions(f.pool), postgres.NewDraftsRepo(f.pool), f.loader)
	create := func(user int) error {
		_, err := uc.CreateDraft(f.userContext(user), &contestsmodel.ContestDraftCreation{ContestID: f.contest, Code: "print(1)"})
		return err
	}
	results := make([]<-chan error, 8)
	for i := range results {
		results[i] = asyncConcurrent(func() error { return create(1) })
	}
	awaitConcurrent(t, asyncConcurrent(func() error { return create(2) }))
	require.NoError(t, release(true))
	success := 0
	for _, result := range results {
		select {
		case err := <-result:
			if err == nil {
				success++
			} else {
				require.ErrorIs(t, err, pkg.ErrInvalid)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("draft creation blocked")
		}
	}
	require.Equal(t, 1, success)
	count, err := postgres.NewDraftsRepo(f.pool).GetDraftsCount(f.ctx, f.contest, f.users[1])
	require.NoError(t, err)
	require.EqualValues(t, 50, count)
}

func TestCompletionChecksDeadlineAfterLockWait(t *testing.T) {
	t.Parallel()
	f := newConcurrencyFixture(t)
	job := f.job(t, 1, 10)
	_, err := f.pool.Exec(f.ctx, "UPDATE judge_jobs SET deadline_at=clock_timestamp()+interval '200 milliseconds' WHERE id=$1", job.ID)
	require.NoError(t, err)
	release := holdSubmission(t, f.blockers, job.SubmissionID)
	result := asyncConcurrent(func() error { return f.complete(job, submissionsmodel.Accepted) })
	f.blockers.waitBlocked(t, f.ctx, 1)
	require.Eventually(t, func() bool {
		var expired bool
		err := f.pool.QueryRow(f.ctx, "SELECT deadline_at<clock_timestamp() FROM judge_jobs WHERE id=$1", job.ID).Scan(&expired)
		return err == nil && expired
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, release(true))
	select {
	case err := <-result:
		require.ErrorIs(t, err, judging.ErrLeaseLost)
	case <-time.After(5 * time.Second):
		t.Fatal("operation blocked behind an independent transaction")
	}
	sub, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Saved, sub.State)
}
