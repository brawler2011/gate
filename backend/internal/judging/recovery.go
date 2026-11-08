package judging

import (
	"context"
	"log/slog"
	"time"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
)

const (
	DefaultRecoveryInterval  = 5 * time.Second
	DefaultMaxJobAttempts    = 3
	DefaultRecoveryBatchSize = 50
)

type JudgeRecovery struct {
	jobs         Jobs
	transactions JudgeTransactions
	maxAttempts  int32
	batchSize    int32
	interval     time.Duration
	logger       *slog.Logger
}

func NewJudgeRecovery(
	jobs Jobs,
	transactions JudgeTransactions,
	maxAttempts int32,
	batchSize int32,
	interval time.Duration,
) *JudgeRecovery {
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxJobAttempts
	}
	if batchSize <= 0 {
		batchSize = DefaultRecoveryBatchSize
	}
	if interval <= 0 {
		interval = DefaultRecoveryInterval
	}
	return &JudgeRecovery{
		jobs:         jobs,
		transactions: transactions,
		maxAttempts:  maxAttempts,
		batchSize:    batchSize,
		interval:     interval,
		logger:       slog.Default().With("component", "judge_recovery"),
	}
}

func (r *JudgeRecovery) RecoverJob(ctx context.Context, job judgingmodel.Job) error {
	return r.transactions.Within(ctx, func(tx JudgeTxStore) error {
		if job.Attempt >= r.maxAttempts {
			current, err := tx.FailStalledJob(ctx, job.Ref())
			if err != nil || !current {
				return err
			}
			change, err := tx.UpdateSubmission(ctx, job.SubmissionID, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.GotIE})
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
			return tx.Record(ctx, eventdata.Event{
				AggregateID: sub.ID,
				Type:        eventdata.OutboxEventSubmissionCompleted,
				Payload:     event,
			})
		}

		current, err := tx.RequeueStalledJob(ctx, job.Ref())
		if err != nil || !current {
			return err
		}
		sub, err := tx.GetSubmission(ctx, job.SubmissionID)
		if err != nil {
			return err
		}
		event := sub.CreatedEvent(job.ID, job.Generation, nil)
		return tx.Record(ctx, eventdata.Event{
			AggregateID: sub.ID,
			Type:        eventdata.OutboxEventSubmissionCreated,
			Payload:     event,
		})
	})
}

func (r *JudgeRecovery) RecoverStalledJobs(ctx context.Context) (int, error) {
	stalled, err := r.jobs.PickStalledJobs(ctx, r.batchSize)
	if err != nil {
		return 0, err
	}
	recovered := 0
	for _, job := range stalled {
		if err := r.RecoverJob(ctx, job); err != nil {
			r.logger.ErrorContext(ctx, "failed to recover stalled judge job", "job_id", job.ID, "error", err)
			continue
		}
		recovered++
	}
	return recovered, nil
}

func (r *JudgeRecovery) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	if _, err := r.RecoverStalledJobs(ctx); err != nil {
		r.logger.ErrorContext(ctx, "failed initial stalled jobs recovery", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.InfoContext(ctx, "judge recovery worker stopping...")
			return
		case <-ticker.C:
			if _, err := r.RecoverStalledJobs(ctx); err != nil {
				r.logger.ErrorContext(ctx, "failed stalled jobs recovery", "error", err)
			}
		}
	}
}
