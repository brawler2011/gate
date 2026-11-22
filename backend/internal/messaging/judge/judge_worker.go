package judgeworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	messaging "github.com/brawler2011/gate/backend/internal/messaging"
	"github.com/brawler2011/gate/backend/pkg"

	judging "github.com/brawler2011/gate/backend/internal/judging"
	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

type JudgeWorker struct {
	js          jetstream.JetStream
	judgeUC     JudgeUseCase
	consumer    jetstream.Consumer
	workerCount int
	// Jobs wait in NATS while the host is not ready to judge.
	ready  func() bool
	logger *slog.Logger
	mu          sync.Mutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewJudgeWorker(
	ctx context.Context,
	js jetstream.JetStream,
	judgeUC JudgeUseCase,
	workerCount int,
	ready func() bool,
) (*JudgeWorker, error) {
	const (
		streamName    = "SUBMISSIONS"
		durableName   = "judge_consumer"
		filterSubject = "submissions.created"
	)

	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    -1,
		AckWait:       5 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create judge consumer: %w", err)
	}

	return &JudgeWorker{
		js:          js,
		judgeUC:     judgeUC,
		consumer:    consumer,
		workerCount: workerCount,
		ready:       ready,
		logger:      slog.Default().With("component", "judge_worker"),
	}, nil
}

func (w *JudgeWorker) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	w.mu.Lock()
	w.cancel = cancel
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.cancel = nil
		w.mu.Unlock()
	}()

	w.logger.Info("starting judge worker", "worker_count", w.workerCount)

	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.workerRoutine(runCtx, i)
	}

	<-runCtx.Done()
	w.logger.Info("stopping judge worker, waiting for workers to finish...")

	w.wg.Wait()
	w.logger.Info("judge worker stopped")

	return runCtx.Err()
}

func (w *JudgeWorker) workerRoutine(ctx context.Context, workerID int) {
	defer w.wg.Done()

	logger := w.logger.With("worker_id", workerID)
	logger.Info("worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopping")
			return
		default:
			if !w.ready() {
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
				}
				continue
			}
			msgs, err := w.consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
			if err != nil {
				if errors.Is(err, jetstream.ErrNoMessages) || errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				logger.Error("failed to fetch message", "error", err)
				time.Sleep(1 * time.Second)
				continue
			}

			for msg := range msgs.Messages() {
				w.handleMessage(ctx, msg, logger)
			}
		}
	}
}

func (w *JudgeWorker) handleMessage(ctx context.Context, msg jetstream.Msg, logger *slog.Logger) {
	const handlerTimeout = 5 * time.Minute

	judgeCtx, cancel := context.WithTimeout(messaging.Context(ctx, msg.Headers()), handlerTimeout)
	logger = logger.With("request_id", requestid.Get(judgeCtx))
	diagnosticMeta := queuecontext.Get(judgeCtx)
	if diagnosticMeta.EventID != uuid.Nil {
		logger = logger.With("event_id", diagnosticMeta.EventID)
	}

	if meta, err := msg.Metadata(); err == nil {
		logger = logger.With("delivery_attempt", meta.NumDelivered)
	}

	defer cancel()

	var event submissionsmodel.SubmissionCreatedEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		logger.Error("failed to unmarshal event", "operation", "decode_submission", "error", err)
		if err := msg.Term(); err != nil {
			logger.Error("failed to terminate message", "operation", "nats_term", "error", err)
		}
		return
	}

	if event.Id == uuid.Nil || event.Generation < 1 || event.JobID == uuid.Nil {
		if event.Id != uuid.Nil && event.Generation > 0 && event.JobID == uuid.Nil && event.State == submissionsmodel.Disqualified {
			if err := msg.Ack(); err != nil {
				logger.Error("failed to ack blocked submission event", "error", err)
			}
		} else {
			logger.Error("invalid judge job reference", "operation", "decode_submission")
			if err := msg.Term(); err != nil {
				logger.Error("failed to terminate invalid job event", "error", err)
			}
		}
		return
	}

	logger = logger.With("submission_id", event.Id)
	queuedAt, parseErr := time.Parse(time.RFC3339Nano, msg.Headers().Get(queuecontext.QueuedAtHeader))
	if parseErr == nil && !queuedAt.IsZero() && !queuedAt.After(time.Now()) {
		logger.Info("submission_judging_started", "event", "submission_judging_started", "queue_wait_ms", time.Since(queuedAt).Milliseconds())
	} else {
		reason := "invalid_queued_at"
		if msg.Headers().Get(queuecontext.QueuedAtHeader) == "" {
			reason = "missing_queued_at"
		}
		logger.Info("submission_judging_started", "event", "submission_judging_started", "queue_wait_unavailable_reason", reason)
	}

	startTime := time.Now()
	err := w.runJudge(judgeCtx, msg, judgingmodel.JobRef{ID: event.JobID, SubmissionID: event.Id, Generation: event.Generation})
	duration := time.Since(startTime)

	if err != nil {
		errorKind := "judging_error"
		if errors.Is(err, context.DeadlineExceeded) {
			errorKind = "timeout"
		} else if errors.Is(err, judging.ErrLeaseLost) {
			errorKind = "lease_lost"
		}
		level := slog.LevelError
		if errors.Is(err, judging.ErrBusy) {
			// A duplicate delivery while another worker holds the job.
			level, errorKind = slog.LevelInfo, "busy"
		}
		logger.Log(ctx, level, "submission_judging_failed", "error_kind", errorKind, "event", "submission_judging_failed", "operation", "judge_submission",
			"error", err,
			"duration_ms", duration.Milliseconds(),
		)
		if ackErr := msg.NakWithDelay(5 * time.Second); ackErr != nil {
			logger.Error("failed to retry judging message", "operation", "nats_nak", "error", ackErr)
		}
		return
	}

	logger.Info("submission_judging_completed", "event", "submission_judging_completed",
		"duration_ms", duration.Milliseconds(),
	)

	if err := msg.Ack(); err != nil {
		logger.Error("failed to ack message", "operation", "nats_ack", "error", err)
	}
}

func (w *JudgeWorker) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	w.mu.Unlock()
	if cancel == nil {
		return
	}

	w.logger.Info("stop signal received")
	cancel()
}

func (w *JudgeWorker) runJudge(ctx context.Context, msg jetstream.Msg, reference judgingmodel.JobRef) (resultErr error) {
	defer func() {
		if r := recover(); r != nil {
			resultErr = fmt.Errorf("judging handler panicked: %v\n%s", r, debug.Stack())
		}
	}()
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := msg.InProgress(); err != nil {
					cancel(fmt.Errorf("extend NATS ack deadline: %w", err))
					return
				}
			}
		}
	}()
	defer func() { close(done); <-stopped }()
	return pkg.WithCause(runCtx, w.judgeUC.JudgeSubmission(runCtx, reference))
}
