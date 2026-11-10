package judging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
)

type JudgeUseCase struct {
	jobs            Jobs
	transactions    JudgeTransactions
	submissionsRepo SubmissionsRepo
	packageLoader   *PackageLoader
	executor        *PackageJudge
	eventPublisher  Progress
	logger          *slog.Logger
}

func NewJudgeUseCase(jobs Jobs, transactions JudgeTransactions, submissionsRepo SubmissionsRepo, loader *PackageLoader, executor *PackageJudge, eventPublisher Progress) *JudgeUseCase {
	return &JudgeUseCase{jobs: jobs, transactions: transactions, submissionsRepo: submissionsRepo, packageLoader: loader, executor: executor, eventPublisher: eventPublisher, logger: slog.Default().With("component", "judge_usecase")}
}

func (uc *JudgeUseCase) JudgeSubmission(ctx context.Context, reference judgingmodel.JobRef) error {
	if reference.ID == uuid.Nil || reference.SubmissionID == uuid.Nil || reference.Generation < 1 {
		return pkg.ErrBadInput
	}
	submissionID := reference.SubmissionID
	job, err := uc.jobs.Claim(ctx, reference)
	if err != nil {
		return err
	}
	if job.ID == uuid.Nil {
		return nil
	}
	leaseCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopped := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				if err := uc.jobs.Renew(leaseCtx, job); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		close(done)
		<-stopped
		releaseCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		if err := uc.jobs.Release(releaseCtx, job); err != nil {
			uc.logger.ErrorContext(releaseCtx, "release judge job", "error", err, "job_id", job.ID)
		}
	}()
	ctx = leaseCtx
	logger := uc.logger.With("submission_id", submissionID, "request_id", requestid.Get(ctx))
	if meta := queuecontext.Get(ctx); meta.EventID != uuid.Nil {
		logger = logger.With("event_id", meta.EventID)
	}
	submission, err := uc.submissionsRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("failed to get submission: %w", err)
	}

	meta := submissionsmodel.SubmissionEventMeta{
		Revision:     submission.Revision,
		UserId:       submission.CreatedBy,
		Username:     submission.Username,
		ContestId:    submission.ContestID,
		ContestTitle: submission.ContestTitle,
		ProblemId:    submission.ProblemID,
		ProblemTitle: submission.ProblemTitle,
		Position:     submission.Position,
		Language:     submission.Language,
		CreatedAt:    submission.CreatedAt,
		UpdatedAt:    submission.UpdatedAt,
	}

	if err := uc.eventPublisher.PublishQueued(ctx, submissionID, meta); err != nil {
		logger.Error("failed to publish queued event", "operation", "judge_submission", "error_type", fmt.Sprintf("%T", err))
	}

	if submission.ProblemID == nil {
		return uc.markInternalError(ctx, job, fmt.Errorf("submission has no problem ID"))
	}

	if job.PackageID == nil {
		return uc.markInternalError(ctx, job, fmt.Errorf("problem has no published version"))
	}
	pkg, err := uc.packageLoader.LoadPackage(ctx, *job.PackageID)
	if err != nil {
		if IsTemporary(err) {
			return err
		}
		return uc.markInternalError(ctx, job, fmt.Errorf("failed to load problem package: %w", err))
	}
	defer pkg.Cleanup()

	if err := uc.eventPublisher.PublishCompilingStarted(ctx, submissionID, meta); err != nil {
		logger.Error("failed to publish compiling started event", "operation", "judge_submission", "error_type", fmt.Sprintf("%T", err))
	}

	verdict, err := uc.executor.Judge(ctx, pkg, submissionID, submission.Submission, submission.Language, uc.eventPublisher, meta)
	if err != nil {
		if IsTemporary(err) {
			return err
		}
		return uc.markInternalError(ctx, job, fmt.Errorf("strategy judging failed: %w", err))
	}

	var failedTest *int32
	if verdict.FailedTest != nil {
		ft := safeInt32(*verdict.FailedTest)
		failedTest = &ft
	}

	if err := uc.applyJudgeResult(ctx, job, &submissionsmodel.SubmissionUpdate{State: verdict.State, Score: verdict.Score, TimeStat: verdict.MaxTime, MemoryStat: verdict.MaxMemory, FailedTest: failedTest, TestDetails: verdict.TestDetails}); err != nil {
		return err
	}

	logger.Info("submission_result", "event", "submission_result",
		"verdict", verdict.State,
		"score", verdict.Score,
		"time", verdict.MaxTime,
		"memory", verdict.MaxMemory,
		"failed_test", failedTest,
	)

	return nil
}

func (uc *JudgeUseCase) markInternalError(ctx context.Context, job judgingmodel.Job, cause error) error {
	uc.logger.ErrorContext(ctx, "judging terminal failure", "job_id", job.ID, "error_type", fmt.Sprintf("%T", cause))
	return uc.applyJudgeResult(ctx, job, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.GotIE})
}

func (uc *JudgeUseCase) applyJudgeResult(ctx context.Context, job judgingmodel.Job, update *submissionsmodel.SubmissionUpdate) error {
	return uc.transactions.Within(ctx, func(tx JudgeTxStore) error {
		current, err := tx.CompleteJob(ctx, job)
		if err != nil {
			return err
		}
		if !current {
			return nil
		}
		change, err := tx.UpdateSubmission(ctx, job.SubmissionID, update)
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
		event := sub.CompletedEvent(change.PreviousState)
		return tx.Record(ctx, eventdata.Event{AggregateID: sub.ID, Type: eventdata.OutboxEventSubmissionCompleted, Payload: event})
	})
}
