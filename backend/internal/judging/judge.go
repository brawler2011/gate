package judging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
)

type JudgeUseCase struct {
	jobs            Jobs
	transactions    JudgeTransactions
	submissionsRepo SubmissionsRepo
	packageLoader   *PackageLoader
	executor        *PackageJudge
	progress        Progress
	logger          *slog.Logger
}

func NewJudgeUseCase(jobs Jobs, transactions JudgeTransactions, submissionsRepo SubmissionsRepo, loader *PackageLoader, executor *PackageJudge, progress Progress) *JudgeUseCase {
	return &JudgeUseCase{jobs: jobs, transactions: transactions, submissionsRepo: submissionsRepo, packageLoader: loader, executor: executor, progress: progress, logger: slog.Default().With("component", "judge_usecase")}
}

// JudgeSubmission judges a claimed job and releases it unless the verdict is stored.
func (uc *JudgeUseCase) JudgeSubmission(ctx context.Context, job judgingmodel.Job) (err error) {
	submissionID := job.SubmissionID
	leaseCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	defer func() { err = pkg.WithCause(leaseCtx, err) }()
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
					cancel(fmt.Errorf("renew judge job lease: %w", err))
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
	logger := uc.logger.With("submission_id", submissionID, "job_id", job.ID)
	submission, err := uc.submissionsRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("failed to get submission: %w", err)
	}

	if err := uc.progress.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventQueued, ID: submissionID}); err != nil {
		logger.Error("failed to publish queued event", "operation", "judge_submission", "error", err)
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
	defer pkg.Release()

	if err := uc.progress.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCompilingStarted, ID: submissionID}); err != nil {
		logger.Error("failed to publish compiling started event", "operation", "judge_submission", "error", err)
	}

	verdict, err := uc.executor.Judge(ctx, pkg, submissionID, submission.Submission, submission.Language, uc.progress)
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

	if err := uc.applyJudgeResult(ctx, job, &submissionsmodel.SubmissionUpdate{State: verdict.State, Score: verdict.Score, TimeStat: verdict.MaxTime, MemoryStat: verdict.MaxMemory, FailedTest: failedTest, TestDetails: verdict.TestDetails, Calibration: verdict.Calibration}); err != nil {
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
	uc.logger.ErrorContext(ctx, "judging terminal failure", "job_id", job.ID, "submission_id", job.SubmissionID, "error", cause)
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
		if _, err := tx.UpdateSubmission(ctx, job.SubmissionID, update); err != nil {
			return err
		}
		sub, err := tx.GetSubmission(ctx, job.SubmissionID)
		if err != nil {
			return err
		}
		return tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCompleted, ID: sub.ID})
	})
}
