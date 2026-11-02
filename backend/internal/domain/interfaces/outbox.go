package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/events"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OutboxRepo interface {
	events.Writer
	DeleteOldEvents(ctx context.Context, retentionDays int32) error
	PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]models.OutboxEvent, error)
	CreateEvent(ctx context.Context, event *models.CreateOutboxEventParams) error
	MarkAsCompleted(ctx context.Context, id, leaseToken uuid.UUID) error
	MarkAsFailed(ctx context.Context, id, leaseToken uuid.UUID, errorMsg string, permanent bool) error
	WithTx(tx pgx.Tx) OutboxRepo
}
