//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	judgeworker "github.com/brawler2011/gate/backend/internal/messaging/judge"
	outbox "github.com/brawler2011/gate/backend/internal/messaging/outbox"
	pubsub "github.com/brawler2011/gate/backend/internal/messaging/pubsub"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

type silentJudgeProgress struct{}

func (silentJudgeProgress) PublishQueued(context.Context, uuid.UUID, submissionsmodel.SubmissionEventMeta) error {
	return nil
}

func (silentJudgeProgress) PublishCompilingStarted(context.Context, uuid.UUID, submissionsmodel.SubmissionEventMeta) error {
	return nil
}

func (silentJudgeProgress) PublishTestingStarted(context.Context, uuid.UUID, submissionsmodel.SubmissionEventMeta) error {
	return nil
}

func (silentJudgeProgress) PublishTestStarted(context.Context, uuid.UUID, int32, submissionsmodel.SubmissionEventMeta) error {
	return nil
}

func (s *IntegrationTestSuite) newJobSubmission() (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	suffix := uuid.NewString()[:8]
	user := s.createUser("job_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("job-"+suffix, "Jobs", user.Id)
	problem, contest := uuid.New(), uuid.New()
	s.Require().NoError(s.problemsRepo.CreateProblem(s.ctx, &problemsmodel.CreateProblemParams{ID: problem, OrganizationID: org.ID, Title: "Job", ShortName: "job", Visibility: problemsmodel.ProblemVisibilityPrivate, OwnerID: &user.Id}))
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{ID: contest, OrganizationID: org.ID, Title: "Jobs", Login: "jobs", Visibility: contestsmodel.ContestVisibilityPublic}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: contest, UserId: user.Id, Role: contestsmodel.ContestRoleParticipant}))
	submission, err := pgstore.NewSubmissionsRepo(s.dbPool).CreateSubmission(s.ctx, &submissionsmodel.SubmissionCreation{ContestId: contest, ProblemId: problem, UserId: user.Id, Language: submissionsmodel.Python, Solution: "print(1)"})
	s.Require().NoError(err)
	return submission, contest, problem, user.Id
}

func (s *IntegrationTestSuite) TestJudgeJobsFenceDuplicateLeasesAndGenerations() {
	sub, contest, _, _ := s.newJobSubmission()
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	first, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	_, err = jobs.ScheduleJob(s.ctx, sub)
	s.Require().Error(err, "one job per generation")
	var wg sync.WaitGroup
	claims := make(chan judgingmodel.Job, 2)
	failures := make(chan error, 2)
	for range 2 {
		wg.Go(func() { job, err := jobs.Claim(s.ctx, first.Ref()); claims <- job; failures <- err })
	}
	wg.Wait()
	close(claims)
	close(failures)
	var active judgingmodel.Job
	for job := range claims {
		if job.ID != uuid.Nil {
			active = job
		}
	}
	s.Equal(first.ID, active.ID)
	busy := 0
	for err := range failures {
		if err != nil {
			s.ErrorIs(err, judging.ErrBusy)
			busy++
		}
	}
	s.Equal(1, busy)
	s.Require().NoError(jobs.Renew(s.ctx, active))
	_, err = s.dbPool.Exec(s.ctx, "UPDATE judge_jobs SET deadline_at=now()-interval '1 second' WHERE id=$1", active.ID)
	s.Require().NoError(err)
	replacement, err := jobs.Claim(s.ctx, first.Ref())
	s.Require().NoError(err)
	s.EqualValues(2, replacement.Attempt)
	s.NotEqual(active.LeaseToken, replacement.LeaseToken)
	s.ErrorIs(jobs.Renew(s.ctx, active), judging.ErrLeaseLost)
	s.Require().NoError(jobs.Release(s.ctx, active))
	s.Require().NoError(jobs.Renew(s.ctx, replacement), "a stale release cannot clear the replacement lease")
	txs := pgstore.NewJudgeTransactions(s.dbPool)
	s.Require().NoError(txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		current, err := tx.CompleteJob(s.ctx, active)
		s.False(current)
		return err
	}))
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	_, err = repo.ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest, SubmissionID: &sub})
	s.Require().NoError(err)
	next, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	s.EqualValues(2, next.Generation)
	s.NotEqual(first.ID, next.ID)
	s.Require().NoError(txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		current, err := tx.CompleteJob(s.ctx, replacement)
		s.False(current, "rejudge must reject the old generation")
		return err
	}))
	stale, err := jobs.Claim(s.ctx, first.Ref())
	s.Require().NoError(err)
	s.Equal(uuid.Nil, stale.ID, "an old message must not start the current generation")
	forged := next.Ref()
	forged.Generation++
	ignored, err := jobs.Claim(s.ctx, forged)
	s.Require().NoError(err)
	s.Equal(uuid.Nil, ignored.ID, "job ID and generation must both match")

	current, err := jobs.Claim(s.ctx, next.Ref())
	s.Require().NoError(err)
	s.Equal(next.ID, current.ID)
	s.EqualValues(1, current.Attempt, "stale messages must not consume attempts of the current job")
	s.Require().NoError(repo.BlockSubmission(s.ctx, sub, nil))
	s.ErrorIs(jobs.Renew(s.ctx, current), judging.ErrLeaseLost)
	s.Require().NoError(txs.Within(s.ctx, func(tx judging.JudgeTxStore) error {
		accepted, err := tx.CompleteJob(s.ctx, current)
		s.False(accepted, "blocking must fence active attempts")
		return err
	}))
}

func (s *IntegrationTestSuite) TestJudgeTerminalResultOutboxAndStandingsAreAtomic() {
	sub, contest, problem, user := s.newJobSubmission()
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	scheduled, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.UpsertContestProblemResult(s.ctx, &contestsmodel.UpsertContestProblemResultParams{ContestID: contest, UserID: user, ProblemID: problem, Solved: true}))
	judge := judging.NewJudgeUseCase(jobs, pgstore.NewJudgeTransactions(s.dbPool), pgstore.NewSubmissionsRepo(s.dbPool), nil, "", "", nil, silentJudgeProgress{})
	_, err = s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_terminal_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type='submissions.completed' THEN RAISE EXCEPTION 'injected completion failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_terminal_event BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION reject_terminal_event()`)
	s.Require().NoError(err)
	drop := func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER IF EXISTS reject_terminal_event ON outbox_events; DROP FUNCTION IF EXISTS reject_terminal_event()`)
		s.Require().NoError(err)
	}
	defer drop()
	s.Error(judge.JudgeSubmission(s.ctx, scheduled.Ref()))
	saved, err := pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(submissionsmodel.Saved, saved.State)
	s.EqualValues(1, saved.Revision)
	standings, err := s.contestsRepo.GetContestProblemResult(s.ctx, contest, user, problem)
	s.Require().NoError(err)
	s.True(standings.Solved)
	var jobState string
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT state FROM judge_jobs WHERE submission_id=$1", sub).Scan(&jobState))
	s.Equal("pending", jobState)
	drop()
	s.Require().NoError(judge.JudgeSubmission(s.ctx, scheduled.Ref()))
	saved, err = pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(submissionsmodel.GotIE, saved.State)
	s.EqualValues(2, saved.Revision)
	standings, err = s.contestsRepo.GetContestProblemResult(s.ctx, contest, user, problem)
	s.Require().NoError(err)
	s.False(standings.Solved)
	s.Require().NoError(judge.JudgeSubmission(s.ctx, scheduled.Ref()), "duplicate completion has no effect")
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submissions.completed'", sub).Scan(&count))
	s.Equal(1, count)
}

func (s *IntegrationTestSuite) TestJudgeJobAndOutboxRollbackTogether() {
	sub, _, _, _ := s.newJobSubmission()
	failure := &eventdata.PermanentError{Cause: judging.ErrBusy}
	txs := pgstore.NewSubmissionTransactions(s.dbPool)
	err := txs.Within(s.ctx, func(tx submissions.SubmissionTxStore) error {
		if _, err := tx.ScheduleJob(s.ctx, sub); err != nil {
			return err
		}
		if err := tx.Record(s.ctx, eventdata.Event{AggregateID: sub, Type: eventdata.OutboxEventSubmissionCreated, Payload: submissionsmodel.SubmissionCreatedEvent{Id: sub}}); err != nil {
			return err
		}
		return failure
	})
	s.Same(failure, err)
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM judge_jobs WHERE submission_id=$1", sub).Scan(&count))
	s.Zero(count)
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1", sub).Scan(&count))
	s.Zero(count)
}

func (s *IntegrationTestSuite) TestJudgePackageIsFixedForSubmission() {
	sub, contest, problem, user := s.newJobSubmission()
	original, err := s.problemsRepo.GetProblemById(s.ctx, problem)
	s.Require().NoError(err)
	packages := pgstore.NewPackagesRepo(s.dbPool)
	first, err := packages.CreatePackage(s.ctx, &problemsmodel.CreatePackageParams{ID: uuid.New(), ProblemID: problem, OrganizationID: original.OrganizationID, PackageHash: strings.Repeat("a", 64), Status: "ready"})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{ContestId: contest, ProblemId: problem, PackageId: first.ID}))
	sub, err = pgstore.NewSubmissionsRepo(s.dbPool).CreateSubmission(s.ctx, &submissionsmodel.SubmissionCreation{ContestId: contest, ProblemId: problem, UserId: user, Language: submissionsmodel.Python, Solution: "print(1)"})
	s.Require().NoError(err)
	jobs := pgstore.NewJudgeJobs(s.dbPool)
	initial, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(strings.Repeat("a", 64), initial.PackageHash)
	second, err := packages.CreatePackage(s.ctx, &problemsmodel.CreatePackageParams{ID: uuid.New(), ProblemID: problem, OrganizationID: original.OrganizationID, PackageHash: strings.Repeat("b", 64), Status: "ready"})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.UpdateContestProblemPackage(s.ctx, contest, problem, second.ID))
	claim, err := jobs.Claim(s.ctx, initial.Ref())
	s.Require().NoError(err)
	s.Equal(strings.Repeat("a", 64), claim.PackageHash)
	_, err = pgstore.NewSubmissionsRepo(s.dbPool).ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest, SubmissionID: &sub})
	s.Require().NoError(err)
	next, err := jobs.ScheduleJob(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(strings.Repeat("a", 64), next.PackageHash)
	s.EqualValues(2, next.Generation)
}

func (s *IntegrationTestSuite) TestDurableJudgeCompletionThroughJetStream() {
	sub, _, _, _ := s.newJobSubmission()
	js, _ := newNATSJetStream(s.T())
	s.Require().NoError(pkg.EnsureSubmissionsStream(s.ctx, js))
	tap, err := js.CreateOrUpdateConsumer(s.ctx, pkg.SubmissionsStreamName, jetstream.ConsumerConfig{Durable: "completion_tap", FilterSubject: "submissions.completed", AckPolicy: jetstream.AckExplicitPolicy})
	s.Require().NoError(err)
	txs := pgstore.NewSubmissionTransactions(s.dbPool)
	s.Require().NoError(txs.Within(s.ctx, func(tx submissions.SubmissionTxStore) error {
		job, err := tx.ScheduleJob(s.ctx, sub)
		if err != nil {
			return err
		}
		return tx.Record(s.ctx, eventdata.Event{AggregateID: sub, Type: eventdata.OutboxEventSubmissionCreated, Payload: submissionsmodel.SubmissionCreatedEvent{Id: sub, JobID: job.ID, Generation: job.Generation}})
	}))
	app := judging.NewJudgeUseCase(pgstore.NewJudgeJobs(s.dbPool), pgstore.NewJudgeTransactions(s.dbPool), pgstore.NewSubmissionsRepo(s.dbPool), nil, "", "", nil, judgeworker.NewEventPublisher(js))
	worker, err := judgeworker.NewJudgeWorker(s.ctx, js, app, 1)
	s.Require().NoError(err)
	dispatcher := outbox.NewEventDispatcher()
	publisher := pubsub.NewSubmissionCreatedPublisher(js)
	dispatcher.Register(eventdata.OutboxEventSubmissionCreated, publisher)
	dispatcher.Register(eventdata.OutboxEventSubmissionCompleted, publisher)
	relay := outbox.NewOutboxWorker(dispatcher, pgstore.NewOutboxRepo(s.dbPool))
	ctx, cancel := context.WithCancel(s.ctx)
	var running sync.WaitGroup
	running.Go(func() { _ = worker.Start(ctx) })
	running.Go(func() { relay.Start(ctx) })
	defer func() { cancel(); running.Wait() }()
	message, err := tap.Next(jetstream.FetchMaxWait(15 * time.Second))
	s.Require().NoError(err)
	var completed submissionsmodel.SubmissionCompletedEvent
	s.Require().NoError(json.Unmarshal(message.Data(), &completed))
	s.Equal(sub, completed.Id)
	s.Equal(submissionsmodel.GotIE, completed.State)
	s.EqualValues(2, completed.Revision)
	s.NotEmpty(message.Headers().Get("Nats-Msg-Id"))
	s.Require().NoError(message.Ack())
	saved, err := pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(completed.Revision, saved.Revision)
	s.Equal(completed.State, saved.State)
}

func (s *IntegrationTestSuite) TestJudgeRecoversAfterRelatedEntityDeletion() {
	for _, deleted := range []string{"problem", "contest", "both"} {
		s.Run(deleted, func() {
			sub, contest, problem, _ := s.newJobSubmission()
			jobs := pgstore.NewJudgeJobs(s.dbPool)
			scheduled, err := jobs.ScheduleJob(s.ctx, sub)
			s.Require().NoError(err)
			abandoned, err := jobs.Claim(s.ctx, scheduled.Ref())
			s.Require().NoError(err)
			s.Require().NotEqual(uuid.Nil, abandoned.ID)
			if deleted == "problem" || deleted == "both" {
				_, err = s.dbPool.Exec(s.ctx, "DELETE FROM problems WHERE id=$1", problem)
				s.Require().NoError(err)
			}
			if deleted == "contest" || deleted == "both" {
				_, err = s.dbPool.Exec(s.ctx, "DELETE FROM contests WHERE id=$1", contest)
				s.Require().NoError(err)
			}
			_, err = s.dbPool.Exec(s.ctx, "UPDATE judge_jobs SET deadline_at=now()-interval '1 second' WHERE id=$1", abandoned.ID)
			s.Require().NoError(err)
			judge := judging.NewJudgeUseCase(jobs, pgstore.NewJudgeTransactions(s.dbPool), pgstore.NewSubmissionsRepo(s.dbPool), nil, "", "", nil, silentJudgeProgress{})
			s.Require().NoError(judge.JudgeSubmission(s.ctx, scheduled.Ref()))
			saved, err := pgstore.NewSubmissionsRepo(s.dbPool).GetSubmission(s.ctx, sub)
			s.Require().NoError(err)
			s.Equal(submissionsmodel.GotIE, saved.State)
			s.EqualValues(2, saved.Revision)
			var state string
			var attempt int
			s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT state,attempt FROM judge_jobs WHERE id=$1", scheduled.ID).Scan(&state, &attempt))
			s.Equal("completed", state)
			s.Equal(2, attempt)
			s.Require().NoError(judge.JudgeSubmission(s.ctx, scheduled.Ref()))
			var completedCount int
			s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submissions.completed'", sub).Scan(&completedCount))
			s.Equal(1, completedCount)
		})
	}
}
