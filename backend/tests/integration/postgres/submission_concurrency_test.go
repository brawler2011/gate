//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
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
	permissions      *access.PermissionsUseCase
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
	require.NoError(t, repo.CreateContest(ctx, &contestsmodel.CreateContestParams{ID: f.contest, OrganizationID: org, OwnerID: &f.users[0], Title: "Concurrent", Login: uuid.NewString(), Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &f.start, EndTime: &end}))
	for _, user := range f.users[1:] {
		require.NoError(t, repo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: f.contest, UserId: user, Role: contestsmodel.ContestRoleParticipant}))
	}
	err := dbfixtures.InsertProblem(ctx, f.pool, dbfixtures.ProblemSpec{ID: f.problem, OrganizationID: org, Title: "Problem", ShortName: "a"})
	require.NoError(t, err)
	pack, err := dbfixtures.InsertPackage(ctx, f.pool, f.problem, org, time.Time{})
	require.NoError(t, err)
	require.NoError(t, repo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem, PackageId: pack}))
	f.permissions = access.NewPermissionsUseCase(repo, postgres.NewUsersRepo(f.pool), postgres.NewProblemsRepo(f.pool), postgres.NewTeamsRepo(f.pool), postgres.NewOrganizationsRepo(f.pool))
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
	return submissions.NewSubmissionsUseCase(postgres.NewSubmissionsRepo(f.pool), postgres.NewContestsRepo(f.pool), postgres.NewProblemsRepo(f.pool), tx, f.permissions)
}
func (f *concurrencyFixture) create(tx submissions.SubmissionTransactions, user int) (uuid.UUID, error) {
	return f.submissionUC(tx).CreateSubmission(f.userContext(user), &submissionsmodel.SubmissionCreation{ContestId: f.contest, ProblemId: f.problem, UserId: f.users[user], Solution: "print(1)", Language: submissionsmodel.Python})
}
func (f *concurrencyFixture) job(t *testing.T, user, minute int) judgingmodel.Job {
	t.Helper()
	id, err := f.create(postgres.NewSubmissionTransactions(f.pool), user)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, "UPDATE submissions SET created_at=$2 WHERE id=$1", id, f.start.Add(time.Duration(minute)*time.Minute))
	require.NoError(t, err)
	jobs := postgres.NewJudgeJobs(f.pool)
	var job judgingmodel.Job
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT id,generation FROM judge_jobs WHERE submission_id=$1", id).Scan(&job.ID, &job.Generation))
	job.SubmissionID = id
	job, err = jobs.Claim(f.ctx, job.Ref())
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, job.ID)
	return job
}

// Exercise the complete verdict transaction, including the first standings insert
// and outbox write, rather than just proving a lock can be acquired.
func applyConcurrentVerdict(ctx context.Context, tx judging.JudgeTxStore, job judgingmodel.Job, state submissionsmodel.State) error {
	current, err := tx.CompleteJob(ctx, job)
	if err != nil || !current {
		return err
	}
	change, err := tx.UpdateSubmission(ctx, job.SubmissionID, &submissionsmodel.SubmissionUpdate{State: state})
	if err != nil {
		return err
	}
	sub, err := tx.GetSubmission(ctx, job.SubmissionID)
	if err != nil {
		return err
	}
	if err := contests.ApplySubmissionResult(ctx, tx, &sub); err != nil {
		return err
	}
	return tx.Record(ctx, eventdata.Event{AggregateID: sub.ID, Type: eventdata.OutboxEventSubmissionCompleted, Payload: sub.CompletedEvent(change.PreviousState)})
}
func (f *concurrencyFixture) complete(job judgingmodel.Job, state submissionsmodel.State) error {
	return postgres.NewJudgeTransactions(f.pool).Within(f.ctx, func(tx judging.JudgeTxStore) error { return applyConcurrentVerdict(f.ctx, tx, job, state) })
}

// barrierSubmissionTransactions runs the production command transaction and
// holds it open at barrier once the command's work is done.
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

// holdTuple keeps the scoring tuple of a user locked by an open transaction on blockers.
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
			result, err := postgres.NewContestsRepo(f.pool).GetContestProblemResult(f.ctx, f.contest, f.users[2], f.problem)
			require.NoError(t, err)
			require.True(t, result.Solved)
			var events int
			require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submissions.completed'", sub.ID).Scan(&events))
			require.Equal(t, 1, events)
		})
	}
}

func TestConcurrentTupleCompletions(t *testing.T) {
	t.Parallel()
	f := newConcurrencyFixture(t)
	failed, early, late := f.job(t, 1, 5), f.job(t, 1, 10), f.job(t, 1, 20)
	release := f.holdTuple(t, f.blockers, 1)
	results := []<-chan error{
		asyncConcurrent(func() error { return f.complete(late, submissionsmodel.Accepted) }),
		asyncConcurrent(func() error { return f.complete(failed, submissionsmodel.GotWA) }),
		asyncConcurrent(func() error { return f.complete(early, submissionsmodel.Accepted) }),
	}
	f.blockers.waitBlocked(t, f.ctx, 3)
	require.NoError(t, release(true))
	for _, result := range results {
		awaitConcurrent(t, result)
	}
	standing, err := postgres.NewContestsRepo(f.pool).GetContestProblemResult(f.ctx, f.contest, f.users[1], f.problem)
	require.NoError(t, err)
	require.True(t, standing.Solved)
	require.EqualValues(t, 1, standing.FailedAttempts)
	require.WithinDuration(t, f.start.Add(10*time.Minute), *standing.FirstACTime, time.Millisecond)
	require.EqualValues(t, 10, *standing.TimeMinutes)
	scoreboard, err := contests.NewContestsUseCase(postgres.NewContestsRepo(f.pool), nil, nil, nil).ProjectScoreboard(f.ctx, f.contest, true)
	require.NoError(t, err)
	require.EqualValues(t, 30, scoreboard.Items[0].TotalPenalty)
	var events int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=ANY($1::uuid[]) AND event_type='submissions.completed'`, []uuid.UUID{failed.SubmissionID, early.SubmissionID, late.SubmissionID}).Scan(&events))
	require.Equal(t, 3, events)
}

func TestCompletionRacesAdministrativeMutation(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"rejudge", "block submission", "block problem"} {
		t.Run(mutation, func(t *testing.T) {
			f := newConcurrencyFixture(t)
			job := f.job(t, 1, 10)
			release := f.holdTuple(t, f.blockers, 1)
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
					return contests.NewContestsUseCase(postgres.NewContestsRepo(f.pool), nil, postgres.NewContestCommandTransactions(f.pool), f.permissions).BlockProblemForUser(f.userContext(0), f.contest, f.users[1], f.problem, nil)
				}
			})
			f.blockers.waitBlocked(t, f.ctx, 2)
			require.NoError(t, release(true))
			awaitConcurrent(t, completion)
			awaitConcurrent(t, update)
			// A late result from the old generation has no effect regardless of race order.
			require.NoError(t, f.complete(job, submissionsmodel.Accepted))
			sub, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
			require.NoError(t, err)
			expected := submissionsmodel.Disqualified
			if mutation == "rejudge" {
				expected = submissionsmodel.Saved
			}
			require.Equal(t, expected, sub.State)
			require.EqualValues(t, 2, sub.Generation)
			standing, err := postgres.NewContestsRepo(f.pool).GetContestProblemResult(f.ctx, f.contest, f.users[1], f.problem)
			require.NoError(t, err)
			require.False(t, standing.Solved)
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
		return contests.NewContestsUseCase(postgres.NewContestsRepo(f.pool), nil, postgres.NewContestCommandTransactions(f.pool), f.permissions).BlockProblemForUser(f.userContext(0), f.contest, f.users[1], f.problem, nil)
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
	// Bulk commands lock all candidate tuples in key order. Hold both tuples and
	// find the first one by which holder the commands wait for: while waiting
	// there, they hold no tuple lock, so the other tuple can be released.
	blockers := map[int]blockerPool{1: newBlockerPool(t), 2: newBlockerPool(t)}
	releases := map[int]func(bool) error{}
	for user, pool := range blockers {
		releases[user] = f.holdTuple(t, pool, user)
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
	// Both commands have captured their candidates before waiting. A later insert
	// into another tuple proceeds and must remain outside those bulk commands.
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
	untouched, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, created)
	require.NoError(t, err)
	require.EqualValues(t, 1, untouched.Generation)
	require.EqualValues(t, 1, untouched.Revision)
	sentinel := errors.New("rollback bulk rejudge")
	err = postgres.NewSubmissionTransactions(f.pool).Within(f.ctx, func(tx submissions.SubmissionTxStore) error {
		changed, err := tx.ResetSubmissionsState(f.ctx, submissionsmodel.RejudgeFilter{ContestID: f.contest})
		if err != nil {
			return err
		}
		for _, change := range changed {
			job, err := tx.ScheduleJob(f.ctx, change.ID)
			if err != nil {
				return err
			}
			if err := tx.Record(f.ctx, eventdata.Event{AggregateID: change.ID, Type: eventdata.OutboxEventSubmissionCreated, Payload: job}); err != nil {
				return err
			}
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	for _, job := range jobs {
		sub, err := postgres.NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
		require.NoError(t, err)
		require.EqualValues(t, 3, sub.Generation)
		require.EqualValues(t, 3, sub.Revision)
		var count int
		require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM judge_jobs WHERE submission_id=$1", sub.ID).Scan(&count))
		require.Equal(t, 3, count)
		require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1", sub.ID).Scan(&count))
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
	uc := contests.NewDraftsUseCase(postgres.NewDraftTransactions(f.pool), postgres.NewDraftsRepo(f.pool), f.permissions)
	create := func(user int) error {
		_, err := uc.CreateDraft(f.userContext(user), &contestsmodel.ContestDraftCreation{ContestID: f.contest, UserID: f.users[user], Code: "print(1)"})
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
				require.ErrorIs(t, err, pkg.ErrBadInput)
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
	release := f.holdTuple(t, f.blockers, 1)
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
	var events int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submissions.completed'", job.SubmissionID).Scan(&events))
	require.Zero(t, events, fmt.Sprint(sub))
}
