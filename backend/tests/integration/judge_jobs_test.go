//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type silentJudgeProgress struct{}

func (silentJudgeProgress) Notify(context.Context, eventdata.Notice) error { return nil }

func (s *IntegrationTestSuite) claimNext(jobs *pgstore.JudgeJobs) judgingmodel.Job {
	job, ok, err := jobs.ClaimNext(s.ctx)
	s.Require().NoError(err)
	s.Require().True(ok, "a pending job")
	return job
}

func (s *IntegrationTestSuite) expireJob(id uuid.UUID) {
	_, err := s.dbPool.Exec(s.ctx, "UPDATE judge_jobs SET deadline_at=now()-interval '1 second' WHERE id=$1", id)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) judgeJobCount(sub uuid.UUID) int {
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM judge_jobs WHERE submission_id=$1", sub).Scan(&count))
	return count
}

func (s *IntegrationTestSuite) newJobSubmission() (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	suffix := uuid.NewString()[:8]
	user := s.createUser("job_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("job-"+suffix, "Jobs", user.Id)
	problem, contest := uuid.New(), uuid.New()
	s.Require().NoError(dbfixtures.InsertProblem(s.ctx, s.dbPool, dbfixtures.ProblemSpec{ID: problem, OrganizationID: org.ID, Owner: user.Id, Title: "Job", ShortName: "job"}))
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contest, OrganizationID: org.ID, Title: "Jobs", Login: "jobs", Visibility: contestsmodel.ContestVisibilityPublic}, s.createUser("job_owner_"+suffix, accessmodel.UserRoleUser).Id))
	pack, err := dbfixtures.InsertPackage(s.ctx, s.dbPool, problem, org.ID, time.Time{})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{ContestId: contest, ProblemId: problem, PackageId: pack}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: contest, UserId: user.Id, Role: accessmodel.ContestParticipant}))
	submission, err := pgstore.NewSubmissionsRepo(s.dbPool).CreateSubmission(s.ctx, &submissionsmodel.SubmissionCreation{Kind: submissionsmodel.KindContest, ContestId: contest, ProblemId: problem, UserId: user.Id, Language: "python3", Solution: "print(1)"})
	s.Require().NoError(err)
	return submission, contest, problem, user.Id
}

func (s *IntegrationTestSuite) TestJudgeJobsFenceDuplicateLeasesAndRevisions() {
	sub, contest, _, _ := s.newJobSubmission()
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	first, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	_, err = jobs.ScheduleJob(s.ctx, sub)
	s.Require().Error(err, "one job per revision")
	var wg sync.WaitGroup
	claims := make(chan judgingmodel.Job, 2)
	for range 2 {
		wg.Go(func() {
			job, ok, err := jobs.ClaimNext(s.ctx)
			s.NoError(err)
			if ok {
				claims <- job
			}
		})
	}
	wg.Wait()
	close(claims)
	s.Require().Len(claims, 1, "concurrent workers claim a job once")
	active := <-claims
	s.Equal(first.ID, active.ID)
	s.Require().NoError(jobs.Renew(s.ctx, active))
	s.expireJob(active.ID)
	recovery := judging.NewJudgeRecovery(jobs, pgstore.NewJudgeTransactions(s.dbPool), 3, 10, time.Second)
	_, err = recovery.RecoverStalledJobs(s.ctx)
	s.Require().NoError(err)
	replacement := s.claimNext(jobs)
	s.EqualValues(2, replacement.Attempt)
	s.NotEqual(active.LeaseToken, replacement.LeaseToken)
	s.ErrorIs(jobs.Renew(s.ctx, active), judging.ErrLeaseLost)
	s.Require().NoError(jobs.Release(s.ctx, active))
	s.Require().NoError(jobs.Renew(s.ctx, replacement), "a stale release cannot clear the replacement lease")
	txs := pgstore.NewJudgeTransactions(s.dbPool)
	s.ErrorIs(txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		current, err := tx.CompleteJob(s.ctx, active)
		s.False(current)
		return err
	}), judging.ErrLeaseLost)
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	_, err = repo.ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest, SubmissionID: &sub})
	s.Require().NoError(err)
	next, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	s.EqualValues(2, next.Revision)
	s.NotEqual(first.ID, next.ID)
	s.Require().NoError(txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		current, err := tx.CompleteJob(s.ctx, replacement)
		s.False(current, "rejudge must reject the old revision")
		return err
	}))
	current := s.claimNext(jobs)
	s.Equal(next.ID, current.ID)
	s.EqualValues(1, current.Attempt)
	s.Require().NoError(repo.BlockSubmission(s.ctx, sub, nil))
	s.ErrorIs(jobs.Renew(s.ctx, current), judging.ErrLeaseLost)
	s.Require().NoError(txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		accepted, err := tx.CompleteJob(s.ctx, current)
		s.False(accepted, "blocking must fence active attempts")
		return err
	}))
}

func (s *IntegrationTestSuite) TestJudgeTerminalResultIsAtomic() {
	sub, _, _, _ := s.newJobSubmission()
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	_, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	judge := judging.NewJudgeUseCase(jobs, pgstore.NewJudgeTransactions(s.dbPool), pgstore.NewSubmissionsRepo(s.dbPool), s.unreadablePackageLoader(), nil, silentJudgeProgress{})
	_, err = s.dbPool.Exec(s.ctx, fmt.Sprintf(`CREATE FUNCTION reject_verdict() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state=%d THEN RAISE EXCEPTION 'injected completion failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_verdict BEFORE UPDATE ON submissions FOR EACH ROW EXECUTE FUNCTION reject_verdict()`, submissionsmodel.GotIE))
	s.Require().NoError(err)
	drop := func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER IF EXISTS reject_verdict ON submissions; DROP FUNCTION IF EXISTS reject_verdict()`)
		s.Require().NoError(err)
	}
	defer drop()
	s.Error(judge.JudgeSubmission(s.ctx, s.claimNext(jobs)))
	saved, err := pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(submissionsmodel.Saved, saved.State)
	s.EqualValues(1, saved.Revision)
	var jobState string
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT state FROM judge_jobs WHERE submission_id=$1", sub).Scan(&jobState))
	s.Equal("pending", jobState)
	_, ok, err := jobs.ClaimNext(s.ctx)
	s.Require().NoError(err)
	s.False(ok, "a released job waits before it can be claimed again")
	_, err = s.dbPool.Exec(s.ctx, "UPDATE judge_jobs SET available_at=now() WHERE submission_id=$1", sub)
	s.Require().NoError(err)
	drop()
	s.Require().NoError(judge.JudgeSubmission(s.ctx, s.claimNext(jobs)))
	saved, err = pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(submissionsmodel.GotIE, saved.State)
	s.EqualValues(2, saved.Revision)
	s.Zero(s.judgeJobCount(sub), "a finished job is deleted")
}

func (s *IntegrationTestSuite) TestJudgePackageIsFixedForSubmission() {
	sub, contest, problem, user := s.newJobSubmission()
	original, err := s.problemsRepo.GetProblemById(s.ctx, problem)
	s.Require().NoError(err)
	packages := pgstore.NewPackagesRepo(s.dbPool)
	first, err := dbfixtures.InsertPackage(s.ctx, s.dbPool, problem, original.OrganizationID, time.Time{})
	s.Require().NoError(err)
	firstHash := s.packageHash(packages, first)
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{ContestId: contest, ProblemId: problem, PackageId: first}))
	sub, err = pgstore.NewSubmissionsRepo(s.dbPool).CreateSubmission(s.ctx, &submissionsmodel.SubmissionCreation{Kind: submissionsmodel.KindContest, ContestId: contest, ProblemId: problem, UserId: user, Language: "python3", Solution: "print(1)"})
	s.Require().NoError(err)
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	initial, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(firstHash, initial.PackageHash)
	second, err := dbfixtures.InsertPackage(s.ctx, s.dbPool, problem, original.OrganizationID, time.Time{})
	s.Require().NoError(err)
	s.NotEqual(firstHash, s.packageHash(packages, second))
	s.Require().NoError(s.contestsRepo.UpdateContestProblemPackage(s.ctx, contest, problem, second))
	claim := s.claimNext(jobs)
	s.Equal(firstHash, claim.PackageHash)
	_, err = pgstore.NewSubmissionsRepo(s.dbPool).ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest, SubmissionID: &sub})
	s.Require().NoError(err)
	next, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(firstHash, next.PackageHash)
	s.EqualValues(2, next.Revision)
}

type switchGate struct{ open atomic.Bool }

func (g *switchGate) Ready() bool                { return g.open.Load() }
func (g *switchGate) Admit(context.Context) bool { return g.open.Load() }

func (s *IntegrationTestSuite) TestJudgeWorkerTakesQueuedJobAndNotifies() {
	sub, _, _, _ := s.newJobSubmission()
	ctx, cancel := context.WithCancel(s.ctx)
	var running sync.WaitGroup
	defer func() { cancel(); running.Wait() }()
	notices := make(chan eventdata.Notice, 10)
	listening := make(chan struct{}, 1)
	running.Go(func() {
		_ = pgstore.Listen(ctx, s.dbPool, pgstore.EventsChannel, func() { listening <- struct{}{} }, func(n *pgconn.Notification) {
			var notice eventdata.Notice
			s.NoError(json.Unmarshal([]byte(n.Payload), &notice))
			notices <- notice
		})
	})
	<-listening
	s.Require().NoError(pgstore.NewSubmissionTransactions(s.dbPool).Within(s.ctx, func(tx submissions.SubmissionTxStore) error {
		_, err := tx.ScheduleJob(s.ctx, sub)
		return err
	}))
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	judge := judging.NewJudgeUseCase(jobs, pgstore.NewJudgeTransactions(s.dbPool), pgstore.NewSubmissionsRepo(s.dbPool), s.unreadablePackageLoader(), nil, pgstore.NewNotifier(s.dbPool))
	gate := &switchGate{}
	wake := make(chan struct{}, 1)
	worker := &judging.Worker{Jobs: jobs, Judge: judge, Workers: 1, Gate: gate, Wake: wake, Logger: slog.Default()}
	running.Go(func() { _ = worker.Run(ctx) })
	time.Sleep(2 * time.Second)
	s.Equal(1, s.judgeJobCount(sub), "a worker that is not ready must leave the job queued")
	gate.open.Store(true)
	wake <- struct{}{}
	deadline := time.After(15 * time.Second)
	for {
		select {
		case notice := <-notices:
			if notice.Type != submissionsmodel.SubmissionEventCompleted {
				continue
			}
			s.Equal(sub, notice.ID)
			saved, err := pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
			s.Require().NoError(err)
			s.Equal(submissionsmodel.GotIE, saved.State)
			s.EqualValues(2, saved.Revision)
			s.Zero(s.judgeJobCount(sub))
			return
		case <-deadline:
			s.FailNow("no completion notice")
		}
	}
}

func (s *IntegrationTestSuite) TestJudgeRecoversAfterRelatedEntityDeletion() {
	// Deleting a contest must not strand its in-flight jobs.
	{
		s.Run("pinned problem cannot be deleted", func() {
			_, _, problem, _ := s.newJobSubmission()
			_, err := s.dbPool.Exec(s.ctx, "DELETE FROM problems WHERE id=$1", problem)
			s.Error(err)
		})
	}
	for _, deleted := range []string{"contest"} {
		s.Run(deleted, func() {
			sub, contest, _, _ := s.newJobSubmission()
			jobs := pgstore.NewJudgeJobs(s.dbPool)
			scheduled, err := jobs.ScheduleJob(s.ctx, sub)
			s.Require().NoError(err)
			abandoned := s.claimNext(jobs)
			s.Require().Equal(scheduled.ID, abandoned.ID)
			if deleted == "contest" {
				_, err = s.dbPool.Exec(s.ctx, "DELETE FROM contests WHERE id=$1", contest)
				s.Require().NoError(err)
			}
			s.expireJob(abandoned.ID)
			_, err = judging.NewJudgeRecovery(jobs, pgstore.NewJudgeTransactions(s.dbPool), 3, 10, time.Second).RecoverStalledJobs(s.ctx)
			s.Require().NoError(err)
			retry := s.claimNext(jobs)
			s.EqualValues(2, retry.Attempt)
			judge := judging.NewJudgeUseCase(jobs, pgstore.NewJudgeTransactions(s.dbPool), pgstore.NewSubmissionsRepo(s.dbPool), s.unreadablePackageLoader(), nil, silentJudgeProgress{})
			s.Require().NoError(judge.JudgeSubmission(s.ctx, retry))
			saved, err := pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
			s.Require().NoError(err)
			s.Equal(submissionsmodel.GotIE, saved.State)
			s.EqualValues(2, saved.Revision)
			s.Zero(s.judgeJobCount(sub))
		})
	}
}

func (s *IntegrationTestSuite) TestJudgeLeaseExpirationDuringApplyResultDoesNotAckSilently() {
	sub, _, _, _ := s.newJobSubmission()
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	scheduled, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	claim := s.claimNext(jobs)
	s.Require().Equal(scheduled.ID, claim.ID)
	s.expireJob(scheduled.ID)

	txs := pgstore.NewJudgeTransactions(s.dbPool)
	err = txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		current, err := tx.CompleteJob(s.ctx, claim)
		s.False(current)
		return err
	})
	s.ErrorIs(err, judging.ErrLeaseLost, "must return ErrLeaseLost instead of silent success")

	saved, err := pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(submissionsmodel.Saved, saved.State, "submission must not silently transition or finalize")
}

func (s *IntegrationTestSuite) TestJudgeStaleRevisionAndBlockedSubmissionAckSafely() {
	sub, contest, _, _ := s.newJobSubmission()
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	first, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)

	claim := s.claimNext(jobs)
	s.Require().Equal(first.ID, claim.ID)

	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	_, err = repo.ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest, SubmissionID: &sub})
	s.Require().NoError(err)

	txs := pgstore.NewJudgeTransactions(s.dbPool)
	// Completing a stale revision returns false without an error.
	var accepted bool
	err = txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		var completeErr error
		accepted, completeErr = tx.CompleteJob(s.ctx, claim)
		return completeErr
	})
	s.Require().NoError(err, "stale revision must safely return nil without error")
	s.False(accepted)

	second, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	claim2 := s.claimNext(jobs)
	s.Require().Equal(second.ID, claim2.ID)

	s.Require().NoError(repo.BlockSubmission(s.ctx, sub, nil))
	err = txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		var completeErr error
		accepted, completeErr = tx.CompleteJob(s.ctx, claim2)
		return completeErr
	})
	s.Require().NoError(err, "blocked submission must safely return nil without error")
	s.False(accepted)
}

func (s *IntegrationTestSuite) TestJudgeRecoveryForStalledJobs() {
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	txs := pgstore.NewJudgeTransactions(s.dbPool)
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	recovery := judging.NewJudgeRecovery(jobs, txs, 3, 10, 5*time.Second)

	// A stalled job below the attempt limit is retried.
	sub1, _, _, _ := s.newJobSubmission()
	job1, err := jobs.ScheduleJob(s.ctx, sub1)
	s.Require().NoError(err)
	claimed1 := s.claimNext(jobs)
	s.EqualValues(1, claimed1.Attempt)
	s.expireJob(job1.ID)

	recoveredCount, err := recovery.RecoverStalledJobs(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, recoveredCount)

	var state1 string
	var attempt1 int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT state, attempt FROM judge_jobs WHERE id=$1", job1.ID).Scan(&state1, &attempt1))
	s.Equal("pending", state1)
	s.Equal(1, attempt1, "attempt preserved for next claim")
	s.EqualValues(2, s.claimNext(jobs).Attempt)

	// A stalled job at the attempt limit ends with an internal error.
	sub2, _, _, _ := s.newJobSubmission()
	job2, err := jobs.ScheduleJob(s.ctx, sub2)
	s.Require().NoError(err)
	claimed2 := s.claimNext(jobs)
	s.Equal(job2.ID, claimed2.ID)
	_, err = s.dbPool.Exec(s.ctx, "UPDATE judge_jobs SET attempt=3, deadline_at=now()-interval '5 seconds' WHERE id=$1", job2.ID)
	s.Require().NoError(err)

	recoveredCount, err = recovery.RecoverStalledJobs(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, recoveredCount)

	s.Zero(s.judgeJobCount(sub2))

	saved2, err := repo.GetSubmission(s.ctx, sub2)
	s.Require().NoError(err)
	s.Equal(submissionsmodel.GotIE, saved2.State, "exhausted attempts must transition to GotIE")

	// Recovering an obsolete revision must preserve the rejudged submission.
	sub3, contest3, _, _ := s.newJobSubmission()
	job3, err := jobs.ScheduleJob(s.ctx, sub3)
	s.Require().NoError(err)
	s.Equal(job3.ID, s.claimNext(jobs).ID)
	_, err = repo.ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest3, SubmissionID: &sub3})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, "UPDATE judge_jobs SET deadline_at=now()-interval '5 seconds' WHERE id=$1", job3.ID)
	s.Require().NoError(err)

	recoveredCount, err = recovery.RecoverStalledJobs(s.ctx)
	s.Require().NoError(err)
	s.Equal(1, recoveredCount)

	// Retiring an obsolete job must not overwrite the submission.
	s.Zero(s.judgeJobCount(sub3))
	saved3, err := repo.GetSubmission(s.ctx, sub3)
	s.Require().NoError(err)
	s.Equal(submissionsmodel.Saved, saved3.State, "rejudged submission must remain Saved for new revision")

	// A pending job of a superseded revision can never be claimed, so recovery deletes it.
	sub4, contest4, _, _ := s.newJobSubmission()
	_, err = jobs.ScheduleJob(s.ctx, sub4)
	s.Require().NoError(err)
	_, err = repo.ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest4, SubmissionID: &sub4})
	s.Require().NoError(err)
	current4, err := jobs.ScheduleJob(s.ctx, sub4)
	s.Require().NoError(err)
	_, err = recovery.RecoverStalledJobs(s.ctx)
	s.Require().NoError(err)
	var remaining []uuid.UUID
	rows, err := s.dbPool.Query(s.ctx, "SELECT id FROM judge_jobs WHERE submission_id=$1", sub4)
	s.Require().NoError(err)
	remaining, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	s.Require().NoError(err)
	s.Equal([]uuid.UUID{current4.ID}, remaining)
}

func (s *IntegrationTestSuite) packageHash(packages *pgstore.PackagesRepo, id uuid.UUID) string {
	pack, err := packages.GetPackageByID(s.ctx, id)
	s.Require().NoError(err)
	return pack.PackageHash
}

// Fixture packages have no archive, forcing internal errors without a sandbox.
func (s *IntegrationTestSuite) unreadablePackageLoader() *judging.PackageLoader {
	return &judging.PackageLoader{Store: s.objectStore, Bucket: s.bucket, Cache: diskCache(s.T()), Packages: s.problemDrafts, Limits: testBuildOptions(s.T()).Limits}
}
