package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// JudgeWorker processes submission judging events
type JudgeWorker struct {
	js          jetstream.JetStream
	judgeUC     interfaces.JudgeUseCase
	consumer    jetstream.Consumer
	workerCount int
	logger      *slog.Logger
	mu          sync.Mutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewJudgeWorker creates a new judge worker
func NewJudgeWorker(
	ctx context.Context,
	js jetstream.JetStream,
	judgeUC interfaces.JudgeUseCase,
	workerCount int,
) (*JudgeWorker, error) {
	const (
		streamName    = "SUBMISSIONS"
		durableName   = "judge_consumer"
		filterSubject = "submissions.created"
	)

	// Create or update consumer
	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,               // maximum 5 delivery attempts
		AckWait:       5 * time.Minute, // 5 minutes to process
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create judge consumer: %w", err)
	}

	return &JudgeWorker{
		js:          js,
		judgeUC:     judgeUC,
		consumer:    consumer,
		workerCount: workerCount,
		logger:      slog.Default().With("component", "judge_worker"),
	}, nil
}

// Start starts the judge worker pool
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

	// Start worker pool
	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.workerRoutine(runCtx, i)
	}

	<-runCtx.Done()
	w.logger.Info("stopping judge worker, waiting for workers to finish...")

	// Wait for all workers to complete
	w.wg.Wait()
	w.logger.Info("judge worker stopped")

	return runCtx.Err()
}

// workerRoutine is the main loop for a single worker
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
			// Fetch next message
			msgs, err := w.consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
			if err != nil {
				if errors.Is(err, jetstream.ErrNoMessages) || errors.Is(err, context.DeadlineExceeded) {
					// No messages available, continue
					continue
				}
				logger.Error("failed to fetch message", "error", err)
				time.Sleep(1 * time.Second) // backoff on error
				continue
			}

			// Process the message
			for msg := range msgs.Messages() {
				w.handleMessage(ctx, msg, logger)
			}
		}
	}
}

// handleMessage processes a single submission.created message
func (w *JudgeWorker) handleMessage(ctx context.Context, msg jetstream.Msg, logger *slog.Logger) {
	const handlerTimeout = 5 * time.Minute

	judgeCtx, cancel := context.WithTimeout(requestid.With(context.WithoutCancel(ctx), msg.Headers().Get(requestid.Header)), handlerTimeout)
	logger = logger.With("request_id", requestid.Get(judgeCtx))
	diagnosticMeta := queuecontext.Metadata{}
	if id, err := uuid.Parse(msg.Headers().Get(queuecontext.EventIDHeader)); err == nil && id != uuid.Nil {
		logger = logger.With("event_id", id)
		diagnosticMeta.EventID = id
	}
	if meta, err := msg.Metadata(); err == nil {
		logger = logger.With("delivery_attempt", meta.NumDelivered)
	}

	defer cancel()

	// Parse event
	var event models.SubmissionCreatedEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		logger.Error("failed to unmarshal event", "operation", "decode_submission", "error_type", fmt.Sprintf("%T", err))
		if err := msg.Nak(); err != nil {
			logger.Error("failed to nak message", "operation", "nats_nak", "error_type", fmt.Sprintf("%T", err))
		}
		return
	}

	logger = logger.With("submission_id", event.Id)
	queuedAt, parseErr := time.Parse(time.RFC3339Nano, msg.Headers().Get(queuecontext.QueuedAtHeader))
	diagnosticMeta.QueuedAt = queuedAt
	judgeCtx = queuecontext.With(judgeCtx, diagnosticMeta)
	if parseErr == nil && !queuedAt.IsZero() && !queuedAt.After(time.Now()) {
		logger.Info("submission_judging_started", "event", "submission_judging_started", "queue_wait_ms", time.Since(queuedAt).Milliseconds())
	} else {
		reason := "invalid_queued_at"
		if msg.Headers().Get(queuecontext.QueuedAtHeader) == "" {
			reason = "missing_queued_at"
		}
		logger.Info("submission_judging_started", "event", "submission_judging_started", "queue_wait_unavailable_reason", reason)
	}

	// Judge the submission
	startTime := time.Now()
	err := w.judgeUC.JudgeSubmission(judgeCtx, event.Id)
	duration := time.Since(startTime)

	if err != nil {
		errorKind := "judging_error"
		if errors.Is(err, context.DeadlineExceeded) {
			errorKind = "timeout"
		}
		logger.Error("submission_judging_failed", "error_kind", errorKind, "event", "submission_judging_failed", "operation", "judge_submission",
			"error_type", fmt.Sprintf("%T", err),
			"duration_ms", duration.Milliseconds(),
		)
		if ackErr := msg.Ack(); ackErr != nil {
			logger.Error("failed to ack message after judging failure", "operation", "nats_ack", "error_type", fmt.Sprintf("%T", ackErr))
		}
		return
	}

	logger.Info("submission_judging_completed", "event", "submission_judging_completed",
		"duration_ms", duration.Milliseconds(),
	)

	// Acknowledge successful processing
	if err := msg.Ack(); err != nil {
		logger.Error("failed to ack message", "operation", "nats_ack", "error_type", fmt.Sprintf("%T", err))
	}
}

// Stop gracefully stops the worker
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
