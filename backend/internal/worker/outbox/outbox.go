package outbox

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
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
			log.Println("Outbox worker stopping...")
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
		slog.Error("failed to pick events", "error", err)
		return
	}

	for _, event := range events {
		w.processEvent(ctx, &event)
	}
}

func (w *OutboxWorker) processEvent(ctx context.Context, event *models.OutboxEvent) {
	handlerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eventTimeoutSec*time.Second)
	defer cancel()

	err := w.dispatcher.Dispatch(handlerCtx, event.EventType, event.Payload)
	if err != nil {
		slog.Error("failed to dispatch event", "event_id", event.Id, "error", err)
		if markErr := w.outboxRepo.MarkAsFailed(handlerCtx, event.Id, err.Error()); markErr != nil {
			slog.Error("failed to mark event as failed", "event_id", event.Id, "error", markErr)
		}
		return
	}

	if markErr := w.outboxRepo.MarkAsCompleted(handlerCtx, event.Id); markErr != nil {
		slog.Error("failed to mark event as completed", "event_id", event.Id, "error", markErr)
	}
}

func (w *OutboxWorker) runMaintenance(ctx context.Context) {
	if err := w.outboxRepo.ResetFailedToPending(ctx, maxRetries, retryDelaySec); err != nil {
		slog.Error("failed to reset failed events", "error", err)
	}

	if err := w.outboxRepo.DeleteOldEvents(ctx, retentionDays); err != nil {
		slog.Error("failed to delete old events", "error", err)
	}
}
