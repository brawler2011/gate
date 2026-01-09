package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OutboxRepo interface {
	DeleteOldEvents(ctx context.Context, retentionDays int32) error
	PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]models.OutboxEvent, error)
	CreateEvent(ctx context.Context, event *models.CreateOutboxEventParams) error
	MarkAsCompleted(ctx context.Context, id uuid.UUID) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error
	ResetFailedToPending(ctx context.Context, maxRetries int32, retryDelaySec int32) error
	WithTx(tx pgx.Tx) OutboxRepo
}
