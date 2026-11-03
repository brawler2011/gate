package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/requestid"
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

func (r *OutboxRepo) WithTx(tx pgx.Tx) *OutboxRepo {
	return &OutboxRepo{
		queries: sqlc.New(tx),
	}
}

func (r *OutboxRepo) Record(ctx context.Context, event eventdata.Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("encode event %s: %w", event.Type, err)
	}
	return r.CreateEvent(ctx, &eventdata.CreateOutboxEventParams{Id: uuid.New(), AggregateID: event.AggregateID, EventType: event.Type, Payload: payload, Headers: map[string]string{requestid.Header: requestid.Get(ctx)}})
}

func (r *OutboxRepo) CreateEvent(ctx context.Context, event *eventdata.CreateOutboxEventParams) error {
	var headersBytes []byte
	if len(event.Headers) > 0 {
		var err error
		headersBytes, err = json.Marshal(event.Headers)
		if err != nil {
			return fmt.Errorf("failed to marshal headers: %w", err)
		}
	} else {
		headersBytes = []byte("{}")
	}

	err := r.queries.InsertEvent(ctx, sqlc.InsertEventParams{
		ID:          event.Id,
		AggregateID: event.AggregateID,
		EventType:   event.EventType,
		Payload:     event.Payload,
		Headers:     headersBytes,
	})
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func (r *OutboxRepo) PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]eventdata.OutboxEvent, error) {
	rows, err := r.queries.PickEvents(ctx, sqlc.PickEventsParams{
		LimitCount: limit,
		TimeoutSec: timeoutSec,
		LeaseToken: uuid.New(),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	events := make([]eventdata.OutboxEvent, len(rows))
	for i, row := range rows {
		events[i] = mapOutboxEvent(row)
	}
	return events, nil
}

func (r *OutboxRepo) MarkAsCompleted(ctx context.Context, id, leaseToken uuid.UUID) error {
	count, err := r.queries.MarkAsCompleted(ctx, sqlc.MarkAsCompletedParams{ID: id, LeaseToken: leaseToken})
	return checkOutboxLease(count, err)
}

func (r *OutboxRepo) MarkAsFailed(ctx context.Context, id, leaseToken uuid.UUID, errorMsg string, permanent bool) error {
	count, err := r.queries.MarkAsFailed(ctx, sqlc.MarkAsFailedParams{ID: id, LeaseToken: leaseToken, ErrorMessage: errorMsg, Permanent: permanent})
	return checkOutboxLease(count, err)
}

func checkOutboxLease(count int64, err error) error {
	if err != nil {
		return HandlePgErr(err)
	}
	if count == 0 {
		return pkg.Wrap(pkg.ErrConflict, nil, "outbox lease no longer owned")
	}
	return nil
}

func (r *OutboxRepo) DeleteOldEvents(ctx context.Context, retentionDays int32) error {
	err := r.queries.DeleteOldEvents(ctx, retentionDays)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func mapOutboxEvent(e sqlc.OutboxEvent) eventdata.OutboxEvent {
	var headers map[string]string
	if len(e.Headers) > 0 {
		if err := json.Unmarshal(e.Headers, &headers); err != nil {
			headers = make(map[string]string)
		}
	}
	if headers == nil {
		headers = make(map[string]string)
	}

	var leaseToken uuid.UUID
	if e.LeaseToken != nil {
		leaseToken = *e.LeaseToken
	}
	return eventdata.OutboxEvent{
		Id:               e.ID,
		AggregateVersion: e.AggregateVersion,
		LeaseToken:       leaseToken,
		NextAttemptAt:    e.NextAttemptAt,
		AggregateID:      e.AggregateID,
		EventType:        e.EventType,
		Payload:          e.Payload,
		Headers:          headers,
		Status:           e.Status,
		RetryCount:       e.RetryCount,
		ErrorMessage:     e.ErrorMessage,
		CreatedAt:        e.CreatedAt,
		ProcessedAt:      e.ProcessedAt,
		LockedAt:         e.LockedAt,
		DeadlineAt:       e.DeadlineAt,
	}
}
