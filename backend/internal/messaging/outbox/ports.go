package outbox

import (
	"context"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
)

type Dispatcher interface {
	Dispatch(ctx context.Context, eventType string, payload []byte) error
}

type EventHandler interface {
	Handle(ctx context.Context, eventType string, payload []byte) error
}

type OutboxRepo interface {
	eventdata.Writer
	DeleteOldEvents(ctx context.Context, retentionDays int32) error
	PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]eventdata.OutboxEvent, error)
	RenewLease(ctx context.Context, id, leaseToken uuid.UUID, timeoutSec int32) error

	MarkAsCompleted(ctx context.Context, id, leaseToken uuid.UUID) error
	MarkAsFailed(ctx context.Context, id, leaseToken uuid.UUID, errorMsg string, permanent bool) error
}
