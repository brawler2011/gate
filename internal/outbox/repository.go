package outbox

import (
	"context"
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// InsertEvent writes a new event to the outbox table
func (r *Repository) InsertEvent(ctx context.Context, event *models.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (aggregate_id, aggregate_type, event_type, payload, status, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		event.AggregateId,
		event.AggregateType,
		event.EventType,
		event.Payload,
		event.Status,
		event.RetryCount,
	).Scan(&event.Id, &event.CreatedAt)

	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

// GetPendingEvents fetches pending events with a limit
func (r *Repository) GetPendingEvents(ctx context.Context, limit int) ([]*models.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_id, aggregate_type, event_type, payload, status, 
		       created_at, processed_at, retry_count, error_message
		FROM outbox_events
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2
	`

	events := make([]*models.OutboxEvent, 0)
	err := r.db.SelectContext(ctx, &events, query, models.OutboxEventStatusPending, limit)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return events, nil
}

// MarkAsProcessing updates the event status to processing
func (r *Repository) MarkAsProcessing(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE outbox_events
		SET status = $1
		WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, models.OutboxEventStatusProcessing, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

// MarkAsCompleted updates the event status to completed and sets processed_at
func (r *Repository) MarkAsCompleted(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE outbox_events
		SET status = $1, processed_at = $2
		WHERE id = $3
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, models.OutboxEventStatusCompleted, now, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

// MarkAsFailed updates the event status to failed, increments retry_count, and stores the error message
func (r *Repository) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	query := `
		UPDATE outbox_events
		SET status = $1, retry_count = retry_count + 1, error_message = $2
		WHERE id = $3
	`

	_, err := r.db.ExecContext(ctx, query, models.OutboxEventStatusFailed, errorMsg, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

// ResetFailedToPending resets failed events back to pending if retry_count < maxRetries
func (r *Repository) ResetFailedToPending(ctx context.Context, maxRetries int) error {
	query := `
		UPDATE outbox_events
		SET status = $1
		WHERE status = $2 AND retry_count < $3
	`

	_, err := r.db.ExecContext(ctx, query, models.OutboxEventStatusPending, models.OutboxEventStatusFailed, maxRetries)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

// DeleteOldEvents removes completed events older than the retention period
func (r *Repository) DeleteOldEvents(ctx context.Context, retentionDays int) error {
	query := `
		DELETE FROM outbox_events
		WHERE status = $1 AND processed_at < $2
	`

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	_, err := r.db.ExecContext(ctx, query, models.OutboxEventStatusCompleted, cutoffTime)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

