package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
)

const (
	// interval to poll for new events
	pollInterval = 1 * time.Second
	// interval to run maintenance tasks
	maintenanceInt = 1 * time.Minute
	// number of events to process in a batch
	batchSize = 10
	// timeout for processing a single event (seconds)
	eventTimeoutSec = 30
	// maximum number of retries for a failed event
	maxRetries = 5
	// delay between retries (seconds)
	retryDelaySec = 60
	// number of days to keep completed events
	retentionDays = 14
)

type OutboxWorker struct {
	dispatcher interfaces.EventDispatcher
	outboxRepo interfaces.OutboxRepo
}

func NewOutboxWorker(dispatcher interfaces.EventDispatcher, outboxRepo interfaces.OutboxRepo) *OutboxWorker {
	return &OutboxWorker{
		dispatcher: dispatcher,
		outboxRepo: outboxRepo,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	pollTicker := time.NewTicker(pollInterval)
	maintenanceTicker := time.NewTicker(maintenanceInt)

	defer pollTicker.Stop()
	defer maintenanceTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Outbox worker stopping...")
			return
		case <-pollTicker.C:
			w.processBatch(ctx)
		case <-maintenanceTicker.C:
			w.runMaintenance(ctx)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	events, err := w.outboxRepo.PickEvents(ctx, batchSize, eventTimeoutSec)
	if err != nil {
		slog.Error("failed to pick events", "operation", "outbox_pick", "error_type", fmt.Sprintf("%T", err))
		return
	}

	for _, event := range events {
		w.processEvent(ctx, &event)
	}
}

func (w *OutboxWorker) processEvent(ctx context.Context, event *models.OutboxEvent) {
	handlerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eventTimeoutSec*time.Second)
	defer cancel()

	handlerCtx = requestid.With(handlerCtx, event.Headers[requestid.Header])
	handlerCtx = queuecontext.With(handlerCtx, queuecontext.Metadata{EventID: event.Id, QueuedAt: event.CreatedAt})
	logger := slog.Default().With("event_id", event.Id, "aggregate_id", event.AggregateID, "request_id", requestid.Get(handlerCtx))

	if event.EventType == models.OutboxEventSubmissionCreated {
		logger = logger.With("submission_id", event.AggregateID)
	}
	err := w.dispatcher.Dispatch(handlerCtx, event.EventType, event.Payload)
	if err != nil {
		logger.Error("failed to dispatch event", "operation", "outbox_dispatch", "error_type", fmt.Sprintf("%T", err))
		if markErr := w.outboxRepo.MarkAsFailed(handlerCtx, event.Id, err.Error()); markErr != nil {
			logger.Error("failed to mark event as failed", "operation", "outbox_mark_failed", "error_type", fmt.Sprintf("%T", markErr))
		}
		return
	}

	if markErr := w.outboxRepo.MarkAsCompleted(handlerCtx, event.Id); markErr != nil {
		logger.Error("failed to mark event as completed", "operation", "outbox_mark_completed", "error_type", fmt.Sprintf("%T", markErr))
	}
}

func (w *OutboxWorker) runMaintenance(ctx context.Context) {
	if err := w.outboxRepo.ResetFailedToPending(ctx, maxRetries, retryDelaySec); err != nil {
		slog.Error("failed to reset failed events", "operation", "outbox_reset", "error_type", fmt.Sprintf("%T", err))
	}

	if err := w.outboxRepo.DeleteOldEvents(ctx, retentionDays); err != nil {
		slog.Error("failed to delete old events", "operation", "outbox_delete_old", "error_type", fmt.Sprintf("%T", err))
	}
}
