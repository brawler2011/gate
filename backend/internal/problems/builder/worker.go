package builder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type Worker struct {
	Pipeline    *Pipeline
	Parallelism int
	Lease       time.Duration
	Poll        time.Duration
	// Builds wait while the host is unfit to judge, since they judge solutions too.
	Gate interface {
		Admit(ctx context.Context) bool
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.Parallelism <= 0 || w.Pipeline.Parallelism <= 0 || w.Lease < 3*time.Second || w.Poll <= 0 {
		return fmt.Errorf("invalid build worker configuration")
	}
	var group sync.WaitGroup
	for range w.Parallelism {
		group.Add(1)
		go func() {
			defer group.Done()
			ticker := time.NewTicker(w.Poll)
			defer ticker.Stop()
			for {
				if ctx.Err() != nil {
					return
				}
				if w.Gate.Admit(ctx) {
					if err := w.runNext(ctx); err != nil && !errors.Is(err, context.Canceled) {
						w.Pipeline.Logger.ErrorContext(ctx, "build worker iteration failed", "error", err)
					}
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	ticker := time.NewTicker(w.Poll)
	defer ticker.Stop()
	defer group.Wait()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.Pipeline.Repo.RecoverBuilds(ctx); err != nil {
				w.Pipeline.Logger.ErrorContext(ctx, "recover builds", "error", err)
			}
		}
	}
}
func (w *Worker) runNext(ctx context.Context) error {
	b, err := w.Pipeline.Repo.ClaimBuild(ctx, w.Lease)
	if err != nil {
		return err
	}
	if b.ID == uuid.Nil {
		return nil
	}
	return w.execute(ctx, b)
}
func (w *Worker) execute(ctx context.Context, b model.ProblemBuild) error {
	deadlineCtx, cancelDeadline := context.WithDeadline(ctx, b.Deadline)
	defer cancelDeadline()
	runCtx, cancel := context.WithCancelCause(deadlineCtx)
	defer cancel(nil)
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(w.Lease / 3)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := w.Pipeline.Repo.HeartbeatBuild(runCtx, b.ID, *b.LeaseToken, w.Lease); err != nil {
					cancel(fmt.Errorf("heartbeat build: %w", err))
					return
				}
			}
		}
	}()
	_, err := w.Pipeline.Execute(runCtx, b)
	close(done)
	<-stopped
	if err == nil {
		return nil
	}
	err = pkg.WithCause(runCtx, err)
	var failure *Failure
	retry := !errors.As(err, &failure) || failure.Kind == "infrastructure"
	level := slog.LevelInfo
	if retry {
		level = slog.LevelError
	}
	w.Pipeline.Logger.Log(ctx, level, "build failed", "build_id", b.ID, "problem_id", b.ProblemID, "retry", retry, "error", err)
	finishCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	closeErr := w.Pipeline.Repo.FinishBuildAttempt(finishCtx, b.ID, *b.LeaseToken, diagnostic(err), retry)
	if errors.Is(closeErr, pkg.ErrConflict) {
		return nil
	} // recovery/cancellation already revoked ownership.
	return closeErr
}
