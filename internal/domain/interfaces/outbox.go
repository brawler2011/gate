package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OutboxRepo interface {
	DeleteOldEvents(ctx context.Context, retentionDays int) error
	GetPendingEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error)
	InsertEvent(ctx context.Context, event *models.OutboxEvent) error
	MarkAsCompleted(ctx context.Context, id uuid.UUID) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error
	MarkAsProcessing(ctx context.Context, id uuid.UUID) error
	ResetFailedToPending(ctx context.Context, maxRetries int) error
	WithTx(tx pgx.Tx) OutboxRepo // FIXME: layers violation
}
