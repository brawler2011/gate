//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/internal/judging"
	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
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
}

func newConcurrencyFixture(t *testing.T) *concurrencyFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	f := &concurrencyFixture{pool: getSharedTestPool(t), ctx: ctx, contest: uuid.New(), problem: uuid.New(), start: time.Now().Add(-time.Hour).Truncate(time.Second)}
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
	repo := NewContestsRepo(f.pool)
	require.NoError(t, repo.CreateContest(ctx, &contestsmodel.CreateContestParams{ID: f.contest, OrganizationID: org, OwnerID: &f.users[0], Title: "Concurrent", Login: uuid.NewString(), Visibility: contestsmodel.ContestVisibilityPublic, StartTime: &f.start, EndTime: &end}))
	for _, user := range f.users[1:] {
		require.NoError(t, repo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: f.contest, UserId: user, Role: contestsmodel.ContestRoleParticipant}))
	}
	_, err := f.pool.Exec(ctx, `INSERT INTO problems(id,organization_id,title,short_name) VALUES ($1,$2,'Problem','a')`, f.problem, org)
	require.NoError(t, err)
	pack := uuid.New()
	hash := strings.ReplaceAll(pack.String(), "-", "")
	_, err = f.pool.Exec(ctx, `INSERT INTO problem_packages(id,problem_id,organization_id,package_hash,status,version) VALUES ($1,$2,$3,$4,'ready',1)`, pack, f.problem, org, hash+hash)
	require.NoError(t, err)
	require.NoError(t, repo.CreateContestProblem(ctx, contestsmodel.ContestProblemCreation{ContestId: f.contest, ProblemId: f.problem, PackageId: pack}))
	f.permissions = access.NewPermissionsUseCase(repo, NewUsersRepo(f.pool), NewProblemsRepo(f.pool), NewTeamsRepo(f.pool), NewOrganizationsRepo(f.pool))
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
	return submissions.NewSubmissionsUseCase(NewSubmissionsRepo(f.pool), NewContestsRepo(f.pool), NewProblemsRepo(f.pool), tx, f.permissions)
}
func (f *concurrencyFixture) create(tx submissions.SubmissionTransactions, user int) (uuid.UUID, error) {
	return f.submissionUC(tx).CreateSubmission(f.userContext(user), &submissionsmodel.SubmissionCreation{ContestId: f.contest, ProblemId: f.problem, UserId: f.users[user], Solution: "print(1)", Language: submissionsmodel.Python})
}
func (f *concurrencyFixture) job(t *testing.T, user, minute int) judgingmodel.Job {
	t.Helper()
	id, err := f.create(NewSubmissionTransactions(f.pool), user)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, "UPDATE submissions SET created_at=$2 WHERE id=$1", id, f.start.Add(time.Duration(minute)*time.Minute))
	require.NoError(t, err)
	jobs := NewJudgeJobs(f.pool)
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
	return NewJudgeTransactions(f.pool).Within(f.ctx, func(tx judging.JudgeTxStore) error { return applyConcurrentVerdict(f.ctx, tx, job, state) })
}

type fixedSubmissionTransaction struct{ submissions.SubmissionTxStore }

func (tx fixedSubmissionTransaction) Within(_ context.Context, fn func(submissions.SubmissionTxStore) error) error {
	return fn(tx.SubmissionTxStore)
}

func beginConcurrencyTx(t *testing.T, f *concurrencyFixture) pgx.Tx {
	t.Helper()
	tx, err := f.pool.Begin(f.ctx)
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
func waitForTupleWaiters(t *testing.T, f *concurrencyFixture, count int) {
	t.Helper()
	waitForAdvisoryWaiters(t, f, submissionTupleKey(f.contest, f.users[1], f.problem), count)
}

func waitForAdvisoryWaiters(t *testing.T, f *concurrencyFixture, key int64, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var n int
		err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objid=$1::oid AND classid=$2::oid`, uint32(uint64(key)), uint32(uint64(key)>>32)).Scan(&n)
		return err == nil && n >= count
	}, 3*time.Second, 5*time.Millisecond)
}

func TestIndependentSubmissionTransactions(t *testing.T) {
	for _, held := range []string{"creation", "completion", "administrative edit"} {
		t.Run(held, func(t *testing.T) {
			f := newConcurrencyFixture(t)
			first, other := f.job(t, 1, 10), f.job(t, 2, 20)
			tx := beginConcurrencyTx(t, f)
			q := sqlc.New(tx)
			switch held {
			case "creation":
				bound := NewSubmissionTransactions(f.pool).(guardedTransactions[submissions.SubmissionTxStore]).transactions.bind(q)
				_, err := f.create(fixedSubmissionTransaction{bound}, 1)
				require.NoError(t, err)
			case "completion":
				bound := NewJudgeTransactions(f.pool).(transactions[judging.JudgeTxStore]).bind(q)
				require.NoError(t, applyConcurrentVerdict(f.ctx, bound, first, submissionsmodel.Accepted))
			default:
				_, err := tx.Exec(f.ctx, "SELECT id FROM contests WHERE id=$1 FOR NO KEY UPDATE", f.contest)
				require.NoError(t, err)
			}
			awaitConcurrent(t, asyncConcurrent(func() error { _, err := f.create(NewSubmissionTransactions(f.pool), 2); return err }))
			awaitConcurrent(t, asyncConcurrent(func() error { return f.complete(other, submissionsmodel.Accepted) }))
			sub, err := NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, other.SubmissionID)
			require.NoError(t, err)
			require.Equal(t, submissionsmodel.Accepted, sub.State)
			require.EqualValues(t, 2, sub.Revision)
			result, err := NewContestsRepo(f.pool).GetContestProblemResult(f.ctx, f.contest, f.users[2], f.problem)
			require.NoError(t, err)
			require.True(t, result.Solved)
			var events int
			require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submissions.completed'", sub.ID).Scan(&events))
			require.Equal(t, 1, events)
		})
	}
}

func TestConcurrentTupleCompletions(t *testing.T) {
	f := newConcurrencyFixture(t)
	failed, early, late := f.job(t, 1, 5), f.job(t, 1, 10), f.job(t, 1, 20)
	tx := beginConcurrencyTx(t, f)
	require.NoError(t, (&ContestsRepo{queries: sqlc.New(tx)}).LockSubmissionTuple(f.ctx, f.contest, f.users[1], f.problem))
	results := []<-chan error{
		asyncConcurrent(func() error { return f.complete(late, submissionsmodel.Accepted) }),
		asyncConcurrent(func() error { return f.complete(failed, submissionsmodel.GotWA) }),
		asyncConcurrent(func() error { return f.complete(early, submissionsmodel.Accepted) }),
	}
	waitForTupleWaiters(t, f, 3)
	require.NoError(t, tx.Commit(f.ctx))
	for _, result := range results {
		awaitConcurrent(t, result)
	}
	standing, err := NewContestsRepo(f.pool).GetContestProblemResult(f.ctx, f.contest, f.users[1], f.problem)
	require.NoError(t, err)
	require.True(t, standing.Solved)
	require.EqualValues(t, 1, standing.FailedAttempts)
	require.WithinDuration(t, f.start.Add(10*time.Minute), *standing.FirstACTime, time.Millisecond)
	require.EqualValues(t, 10, *standing.TimeMinutes)
	scoreboard, err := contests.NewContestsUseCase(NewContestsRepo(f.pool), nil, nil, nil).ProjectScoreboard(f.ctx, f.contest, true)
	require.NoError(t, err)
	require.EqualValues(t, 30, scoreboard.Items[0].TotalPenalty)
	var events int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=ANY($1::uuid[]) AND event_type='submissions.completed'`, []uuid.UUID{failed.SubmissionID, early.SubmissionID, late.SubmissionID}).Scan(&events))
	require.Equal(t, 3, events)
}

func TestCompletionRacesAdministrativeMutation(t *testing.T) {
	for _, mutation := range []string{"rejudge", "block submission", "block problem"} {
		t.Run(mutation, func(t *testing.T) {
			f := newConcurrencyFixture(t)
			job := f.job(t, 1, 10)
			tx := beginConcurrencyTx(t, f)
			require.NoError(t, (&ContestsRepo{queries: sqlc.New(tx)}).LockSubmissionTuple(f.ctx, f.contest, f.users[1], f.problem))
			completion := asyncConcurrent(func() error { return f.complete(job, submissionsmodel.Accepted) })
			uc := f.submissionUC(NewSubmissionTransactions(f.pool))
			update := asyncConcurrent(func() error {
				switch mutation {
				case "rejudge":
					_, err := uc.RejudgeSubmissions(f.userContext(0), submissionsmodel.RejudgeFilter{ContestID: f.contest, SubmissionID: &job.SubmissionID})
					return err
				case "block submission":
					return uc.BlockSubmission(f.userContext(0), f.contest, job.SubmissionID, nil)
				default:
					return contests.NewContestsUseCase(NewContestsRepo(f.pool), nil, NewContestCommandTransactions(f.pool), f.permissions).BlockProblemForUser(f.userContext(0), f.contest, f.users[1], f.problem, nil)
				}
			})
			waitForTupleWaiters(t, f, 2)
			require.NoError(t, tx.Commit(f.ctx))
			awaitConcurrent(t, completion)
			awaitConcurrent(t, update)
			// A late result from the old generation has no effect regardless of race order.
			require.NoError(t, f.complete(job, submissionsmodel.Accepted))
			sub, err := NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
			require.NoError(t, err)
			expected := submissionsmodel.Disqualified
			if mutation == "rejudge" {
				expected = submissionsmodel.Saved
			}
			require.Equal(t, expected, sub.State)
			require.EqualValues(t, 2, sub.Generation)
			standing, err := NewContestsRepo(f.pool).GetContestProblemResult(f.ctx, f.contest, f.users[1], f.problem)
			require.NoError(t, err)
			require.False(t, standing.Solved)
		})
	}
}

func TestCreationRacesProblemBlock(t *testing.T) {
	f := newConcurrencyFixture(t)
	tx := beginConcurrencyTx(t, f)
	require.NoError(t, (&ContestsRepo{queries: sqlc.New(tx)}).LockSubmissionTuple(f.ctx, f.contest, f.users[1], f.problem))
	var id uuid.UUID
	create := asyncConcurrent(func() error { var err error; id, err = f.create(NewSubmissionTransactions(f.pool), 1); return err })
	block := asyncConcurrent(func() error {
		return contests.NewContestsUseCase(NewContestsRepo(f.pool), nil, NewContestCommandTransactions(f.pool), f.permissions).BlockProblemForUser(f.userContext(0), f.contest, f.users[1], f.problem, nil)
	})
	waitForTupleWaiters(t, f, 2)
	require.NoError(t, tx.Commit(f.ctx))
	awaitConcurrent(t, create)
	awaitConcurrent(t, block)
	sub, err := NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, id)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Disqualified, sub.State)
}

func TestBulkRejudgeLockingAndRollback(t *testing.T) {
	f := newConcurrencyFixture(t)
	jobs := []judgingmodel.Job{f.job(t, 1, 10), f.job(t, 2, 20)}
	blocker := beginConcurrencyTx(t, f)
	key := submissionTupleKey(f.contest, f.users[1], f.problem)
	otherKey := submissionTupleKey(f.contest, f.users[2], f.problem)
	if otherKey < key {
		key = otherKey
	}
	require.NoError(t, sqlc.New(blocker).AdvisoryTransactionLock(f.ctx, key))
	uc := f.submissionUC(NewSubmissionTransactions(f.pool))
	results := []<-chan error{}
	for range 2 {
		results = append(results, asyncConcurrent(func() error {
			_, err := uc.RejudgeSubmissions(f.userContext(0), submissionsmodel.RejudgeFilter{ContestID: f.contest})
			return err
		}))
	}
	waitForAdvisoryWaiters(t, f, key, 2)
	// Both commands have captured their candidates before waiting. A later insert
	// into another tuple proceeds and must remain outside those bulk commands.
	newUser := 2
	if key == otherKey {
		newUser = 1
	}
	var created uuid.UUID
	awaitConcurrent(t, asyncConcurrent(func() error {
		var err error
		created, err = f.create(NewSubmissionTransactions(f.pool), newUser)
		return err
	}))
	require.NoError(t, blocker.Commit(f.ctx))
	for _, result := range results {
		awaitConcurrent(t, result)
	}
	untouched, err := NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, created)
	require.NoError(t, err)
	require.EqualValues(t, 1, untouched.Generation)
	require.EqualValues(t, 1, untouched.Revision)
	sentinel := errors.New("rollback bulk rejudge")
	err = NewSubmissionTransactions(f.pool).Within(f.ctx, func(tx submissions.SubmissionTxStore) error {
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
		sub, err := NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
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
	f := newConcurrencyFixture(t)
	_, err := f.pool.Exec(f.ctx, `INSERT INTO contest_drafts(contest_id,user_id,code) SELECT $1,$2,'draft' FROM generate_series(1,49)`, f.contest, f.users[1])
	require.NoError(t, err)
	tx := beginConcurrencyTx(t, f)
	require.NoError(t, (&DraftsRepo{queries: sqlc.New(tx)}).LockDraftOwner(f.ctx, f.contest, f.users[1]))
	uc := contests.NewDraftsUseCase(NewDraftTransactions(f.pool), NewDraftsRepo(f.pool), f.permissions)
	create := func(user int) error {
		_, err := uc.CreateDraft(f.userContext(user), &contestsmodel.ContestDraftCreation{ContestID: f.contest, UserID: f.users[user], Code: "print(1)"})
		return err
	}
	results := make([]<-chan error, 8)
	for i := range results {
		results[i] = asyncConcurrent(func() error { return create(1) })
	}
	awaitConcurrent(t, asyncConcurrent(func() error { return create(2) }))
	require.NoError(t, tx.Commit(f.ctx))
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
	count, err := NewDraftsRepo(f.pool).GetDraftsCount(f.ctx, f.contest, f.users[1])
	require.NoError(t, err)
	require.EqualValues(t, 50, count)
}

func TestCompletionChecksDeadlineAfterLockWait(t *testing.T) {
	f := newConcurrencyFixture(t)
	job := f.job(t, 1, 10)
	_, err := f.pool.Exec(f.ctx, "UPDATE judge_jobs SET deadline_at=clock_timestamp()+interval '200 milliseconds' WHERE id=$1", job.ID)
	require.NoError(t, err)
	tx := beginConcurrencyTx(t, f)
	require.NoError(t, (&ContestsRepo{queries: sqlc.New(tx)}).LockSubmissionTuple(f.ctx, f.contest, f.users[1], f.problem))
	result := asyncConcurrent(func() error { return f.complete(job, submissionsmodel.Accepted) })
	waitForTupleWaiters(t, f, 1)
	require.Eventually(t, func() bool {
		var expired bool
		err := f.pool.QueryRow(f.ctx, "SELECT deadline_at<clock_timestamp() FROM judge_jobs WHERE id=$1", job.ID).Scan(&expired)
		return err == nil && expired
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, tx.Commit(f.ctx))
	awaitConcurrent(t, result)
	sub, err := NewSubmissionsRepo(f.pool).GetSubmission(f.ctx, job.SubmissionID)
	require.NoError(t, err)
	require.Equal(t, submissionsmodel.Saved, sub.State)
	var events int
	require.NoError(t, f.pool.QueryRow(f.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submissions.completed'", job.SubmissionID).Scan(&events))
	require.Zero(t, events, fmt.Sprint(sub))
}

func (t fixedSubmissionTransaction) Prepare(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
