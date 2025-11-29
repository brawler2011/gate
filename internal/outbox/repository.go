package outbox

import (
	"context"
	"time"

	"github.com/gate149/core/internal/models"
	outboxsqlc "github.com/gate149/core/internal/outbox/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	queries *outboxsqlc.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		queries: outboxsqlc.New(db),
	}
}

// InsertEvent writes a new event to the outbox table
func (r *Repository) InsertEvent(ctx context.Context, event *outboxsqlc.OutboxEvent) error {
	row, err := r.queries.InsertEvent(ctx, outboxsqlc.InsertEventParams{
		AggregateID:   event.AggregateID,
		AggregateType: event.AggregateType,
		EventType:     event.EventType,
		Payload:       event.Payload,
		Status:        event.Status,
		RetryCount:    event.RetryCount,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	event.ID = row.ID
	event.CreatedAt = row.CreatedAt
	return nil
}

// GetPendingEvents fetches pending events with a limit
func (r *Repository) GetPendingEvents(ctx context.Context, limit int) ([]outboxsqlc.OutboxEvent, error) {
	rows, err := r.queries.GetPendingEvents(ctx, outboxsqlc.GetPendingEventsParams{
		Status: models.OutboxEventStatusPending,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return rows, nil
}

// MarkAsProcessing updates the event status to processing
func (r *Repository) MarkAsProcessing(ctx context.Context, id uuid.UUID) error {
	err := r.queries.MarkAsProcessing(ctx, outboxsqlc.MarkAsProcessingParams{
		Status: models.OutboxEventStatusProcessing,
		ID:     id,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// MarkAsCompleted updates the event status to completed and sets processed_at
func (r *Repository) MarkAsCompleted(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	err := r.queries.MarkAsCompleted(ctx, outboxsqlc.MarkAsCompletedParams{
		Status:      models.OutboxEventStatusCompleted,
		ProcessedAt: timeToPgtype(now),
		ID:          id,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// MarkAsFailed updates the event status to failed, increments retry_count, and stores the error message
func (r *Repository) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	err := r.queries.MarkAsFailed(ctx, outboxsqlc.MarkAsFailedParams{
		Status:       models.OutboxEventStatusFailed,
		ErrorMessage: textFromString(errorMsg),
		ID:           id,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// ResetFailedToPending resets failed events back to pending if retry_count < maxRetries
func (r *Repository) ResetFailedToPending(ctx context.Context, maxRetries int) error {
	err := r.queries.ResetFailedToPending(ctx, outboxsqlc.ResetFailedToPendingParams{
		Status:     models.OutboxEventStatusPending,
		StatusOld:  models.OutboxEventStatusFailed,
		MaxRetries: int32(maxRetries),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// DeleteOldEvents removes completed events older than the retention period
func (r *Repository) DeleteOldEvents(ctx context.Context, retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	err := r.queries.DeleteOldEvents(ctx, outboxsqlc.DeleteOldEventsParams{
		Status:     models.OutboxEventStatusCompleted,
		BeforeDate: timeToPgtype(cutoffTime),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// Helper functions

func timeToPgtype(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func textFromString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
