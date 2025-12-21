package pg

import (
	"context"
	"time"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/repository/pg/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

func (r *OutboxRepo) InsertEvent(ctx context.Context, event *models.OutboxEvent) error {
	row, err := r.queries.InsertEvent(ctx, sqlc.InsertEventParams{
		AggregateID:   event.AggregateID,
		AggregateType: event.AggregateType,
		EventType:     models.OutboxEventType(event.EventType),
		Payload:       event.Payload,
		Status:        models.OutboxEventStatus(event.Status),
		RetryCount:    event.RetryCount,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	event.ID = row.ID
	event.CreatedAt = row.CreatedAt
	return nil
}

func (r *OutboxRepo) GetPendingEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	rows, err := r.queries.GetPendingEvents(ctx, sqlc.GetPendingEventsParams{
		Status: models.OutboxEventStatusPending,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	events := make([]models.OutboxEvent, len(rows))
	for i, row := range rows {
		events[i] = mapOutboxEvent(row)
	}
	return events, nil
}

func (r *OutboxRepo) MarkAsProcessing(ctx context.Context, id uuid.UUID) error {
	err := r.queries.MarkAsProcessing(ctx, sqlc.MarkAsProcessingParams{
		Status: models.OutboxEventStatusProcessing,
		ID:     id,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *OutboxRepo) MarkAsCompleted(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	err := r.queries.MarkAsCompleted(ctx, sqlc.MarkAsCompletedParams{
		Status:      models.OutboxEventStatusCompleted,
		ProcessedAt: timeToPgtype(now),
		ID:          id,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *OutboxRepo) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	err := r.queries.MarkAsFailed(ctx, sqlc.MarkAsFailedParams{
		Status:       models.OutboxEventStatusFailed,
		ErrorMessage: textFromString(errorMsg),
		ID:           id,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *OutboxRepo) ResetFailedToPending(ctx context.Context, maxRetries int) error {
	err := r.queries.ResetFailedToPending(ctx, sqlc.ResetFailedToPendingParams{
		Status:     models.OutboxEventStatusPending,
		StatusOld:  models.OutboxEventStatusFailed,
		MaxRetries: int32(maxRetries),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *OutboxRepo) DeleteOldEvents(ctx context.Context, retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	err := r.queries.DeleteOldEvents(ctx, sqlc.DeleteOldEventsParams{
		Status:     models.OutboxEventStatusCompleted,
		BeforeDate: timeToPgtype(cutoffTime),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func timeToPgtype(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func textFromString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func mapOutboxEvent(e sqlc.OutboxEvent) models.OutboxEvent {
	var errorMsg string
	if e.ErrorMessage != nil {
		errorMsg = *e.ErrorMessage
	}

	var processedAt *time.Time
	if e.ProcessedAt.Valid {
		processedAt = &e.ProcessedAt.Time
	}

	return models.OutboxEvent{
		ID:            e.ID,
		AggregateID:   e.AggregateID,
		AggregateType: e.AggregateType,
		EventType:     e.EventType,
		Payload:       e.Payload,
		Status:        models.OutboxEventStatus(e.Status),
		RetryCount:    e.RetryCount,
		ErrorMessage:  errorMsg,
		CreatedAt:     e.CreatedAt,
		ProcessedAt:   processedAt,
	}
}
