package judging

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
)

const (
	// Wakeups are best-effort, so idle workers also poll.
	pollInterval = 5 * time.Second
	retryDelay   = 5 * time.Second
	judgeTimeout = 5 * time.Minute
)

// Gate holds jobs in the queue while the host is unfit to judge.
type Gate interface {
	// Ready is cheap enough for idle loops.
	Ready() bool
	// Admit checks the host right before a job.
	Admit(ctx context.Context) bool
}

type Worker struct {
	Jobs    Jobs
	Judge   *JudgeUseCase
	Workers int
	Gate    Gate
	// Wake receives a value when a job may have become pending.
	Wake   <-chan struct{}
	Logger *slog.Logger
}

func (w *Worker) Run(ctx context.Context) error {
	w.Logger.Info("starting judge worker", "worker_count", w.Workers)
	var wg sync.WaitGroup
	for i := range w.Workers {
		wg.Go(func() { w.loop(ctx, w.Logger.With("worker_id", i)) })
	}
	wg.Wait()
	return nil
}

func (w *Worker) loop(ctx context.Context, logger *slog.Logger) {
	for ctx.Err() == nil {
		if !w.Gate.Ready() {
			sleep(ctx, nil, time.Second)
			continue
		}
		job, ok, err := w.Jobs.ClaimNext(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "claim judge job", "error", err)
			sleep(ctx, nil, retryDelay)
			continue
		}
		if !ok {
			sleep(ctx, w.Wake, pollInterval)
			continue
		}
		if !w.Gate.Admit(ctx) {
			if err := w.Jobs.Release(context.WithoutCancel(ctx), job); err != nil {
				logger.ErrorContext(ctx, "release judge job", "job_id", job.ID, "error", err)
			}
			sleep(ctx, nil, retryDelay)
			continue
		}
		if err := w.judge(ctx, job, logger.With("submission_id", job.SubmissionID, "job_id", job.ID, "attempt", job.Attempt)); err != nil {
			sleep(ctx, nil, retryDelay)
		}
	}
}

func (w *Worker) judge(ctx context.Context, job judgingmodel.Job, logger *slog.Logger) (err error) {
	ctx, cancel := context.WithTimeout(ctx, judgeTimeout)
	defer cancel()
	logger.InfoContext(ctx, "submission_judging_started", "event", "submission_judging_started", "queue_wait_ms", time.Since(job.CreatedAt).Milliseconds())
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("judging panicked: %v\n%s", r, debug.Stack())
		}
		if err != nil {
			logger.ErrorContext(ctx, "submission_judging_failed", "event", "submission_judging_failed", "error", err, "duration_ms", time.Since(start).Milliseconds())
			return
		}
		logger.InfoContext(ctx, "submission_judging_completed", "event", "submission_judging_completed", "duration_ms", time.Since(start).Milliseconds())
	}()
	return w.Judge.JudgeSubmission(ctx, job)
}

func sleep(ctx context.Context, wake <-chan struct{}, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-wake:
	case <-timer.C:
	}
}
