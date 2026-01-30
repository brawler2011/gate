package pg

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepo struct {
	queries *sqlc.Queries
}

func NewOutboxRepo(db *pgxpool.Pool) *OutboxRepo {
	return &OutboxRepo{
		queries: sqlc.New(db),
	}
}

func (r *OutboxRepo) WithTx(tx pgx.Tx) interfaces.OutboxRepo {
	return &OutboxRepo{
		queries: sqlc.New(tx),
	}
}

func (r *OutboxRepo) CreateEvent(ctx context.Context, event *models.CreateOutboxEventParams) error {
	err := r.queries.InsertEvent(ctx, sqlc.InsertEventParams{
		ID:          event.Id,
		AggregateID: event.AggregateID,
		EventType:   event.EventType,
		Payload:     event.Payload,
	})
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func (r *OutboxRepo) PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]models.OutboxEvent, error) {
	rows, err := r.queries.PickEvents(ctx, sqlc.PickEventsParams{
		LimitCount: limit,
		TimeoutSec: timeoutSec,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	events := make([]models.OutboxEvent, len(rows))
	for i, row := range rows {
		events[i] = mapOutboxEvent(row)
	}
	return events, nil
}

func (r *OutboxRepo) MarkAsCompleted(ctx context.Context, id uuid.UUID) error {
	err := r.queries.MarkAsCompleted(ctx, id)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *OutboxRepo) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	err := r.queries.MarkAsFailed(ctx, sqlc.MarkAsFailedParams{
		ErrorMessage: errorMsg,
		ID:           id,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *OutboxRepo) ResetFailedToPending(ctx context.Context, maxRetries int32, retryDelaySec int32) error {
	err := r.queries.ResetFailedToPending(ctx, sqlc.ResetFailedToPendingParams{
		MaxRetries:    maxRetries,
		RetryDelaySec: retryDelaySec,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *OutboxRepo) DeleteOldEvents(ctx context.Context, retentionDays int32) error {
	err := r.queries.DeleteOldEvents(ctx, sqlc.DeleteOldEventsParams{
		Status:        models.OutboxEventStatusCompleted,
		RetentionDays: retentionDays,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func mapOutboxEvent(e sqlc.OutboxEvent) models.OutboxEvent {
	return models.OutboxEvent{
		Id:           e.ID,
		AggregateID:  e.AggregateID,
		EventType:    e.EventType,
		Payload:      e.Payload,
		Status:       e.Status,
		RetryCount:   e.RetryCount,
		ErrorMessage: e.ErrorMessage,
		CreatedAt:    e.CreatedAt,
		ProcessedAt:  e.ProcessedAt,
		LockedAt:     e.LockedAt,
		DeadlineAt:   e.DeadlineAt,
	}
}
