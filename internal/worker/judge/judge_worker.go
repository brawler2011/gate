package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/nats-io/nats.go/jetstream"
)

// JudgeWorker processes submission judging events
type JudgeWorker struct {
	js          jetstream.JetStream
	judgeUC     interfaces.JudgeUseCase
	consumer    jetstream.Consumer
	workerCount int
	logger      *slog.Logger
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
		MaxDeliver:    5, // maximum 5 delivery attempts
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
	w.logger.Info("starting judge worker", "worker_count", w.workerCount)

	// Start worker pool
	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.workerRoutine(ctx, i)
	}

	// Wait for context cancellation
	<-ctx.Done()
	w.logger.Info("stopping judge worker, waiting for workers to finish...")

	// Wait for all workers to complete
	w.wg.Wait()
	w.logger.Info("judge worker stopped")

	return ctx.Err()
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
				if err == jetstream.ErrNoMessages || err == context.DeadlineExceeded {
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

	// Create context with timeout for judging
	judgeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), handlerTimeout)
	defer cancel()

	// Parse event
	var event models.SubmissionCreatedEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		logger.Error("failed to unmarshal event", "error", err)
		if err := msg.Nak(); err != nil {
			logger.Error("failed to nak message", "error", err)
		}
		return
	}

	logger.Info("processing submission", "submission_id", event.Id)

	// Judge the submission
	startTime := time.Now()
	err := w.judgeUC.JudgeSubmission(judgeCtx, event.Id)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("judging failed",
			"submission_id", event.Id,
			"error", err,
			"duration", duration,
		)

		// Check if we should retry
		meta, metaErr := msg.Metadata()
		if metaErr == nil && meta.NumDelivered < 3 {
			// Retry with backoff
			if err := msg.NakWithDelay(time.Duration(meta.NumDelivered) * 30 * time.Second); err != nil {
				logger.Error("failed to nak with delay", "error", err)
			}
			return
		}

		// Max retries exceeded, terminate message
		if err := msg.Term(); err != nil {
			logger.Error("failed to terminate message", "error", err)
		}
		return
	}

	logger.Info("judging completed",
		"submission_id", event.Id,
		"duration", duration,
	)

	// Acknowledge successful processing
	if err := msg.Ack(); err != nil {
		logger.Error("failed to ack message", "error", err)
	}
}

// Stop gracefully stops the worker
func (w *JudgeWorker) Stop() {
	w.logger.Info("stop signal received")
	// Context cancellation will trigger shutdown
}
