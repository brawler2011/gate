//go:build integration
// +build integration

package integration

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestOutbox_HeadersPersistence() {
	outboxRepo := pg.NewOutboxRepo(s.dbPool)

	s.Run("CreateEventWithHeaders_PickEventsReturnsHeaders", func() {
		eventID := uuid.New()
		aggregateID := uuid.New()
		headers := map[string]string{
			"X-Request-ID":  "request-123",
			"custom-header": "rojo=1",
		}

		err := outboxRepo.CreateEvent(s.ctx, &models.CreateOutboxEventParams{
			Id:          eventID,
			AggregateID: aggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{"test":"data"}`),
			Headers:     headers,
		})
		s.Require().NoError(err)

		events, err := outboxRepo.PickEvents(s.ctx, 10, 30)
		s.Require().NoError(err)

		var found *models.OutboxEvent
		for _, e := range events {
			if e.Id == eventID {
				found = &e
				break
			}
		}

		s.Require().NotNil(found, "created outbox event should be found in picked events")
		s.Equal(eventID, found.Id)
		s.Equal(aggregateID, found.AggregateID)
		s.Equal(models.OutboxEventSubmissionCreated, found.EventType)
		s.JSONEq(`{"test":"data"}`, string(found.Payload))
		s.NotNil(found.Headers)
		s.Equal(headers["X-Request-ID"], found.Headers["X-Request-ID"])
		s.Equal(headers["custom-header"], found.Headers["custom-header"])

		// Clean up
		err = outboxRepo.MarkAsCompleted(s.ctx, eventID)
		s.Require().NoError(err)
	})

	s.Run("CreateEventWithNilHeaders_PickEventsReturnsEmptyMap", func() {
		eventID := uuid.New()
		aggregateID := uuid.New()

		err := outboxRepo.CreateEvent(s.ctx, &models.CreateOutboxEventParams{
			Id:          eventID,
			AggregateID: aggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{"test":"nil_headers"}`),
			Headers:     nil,
		})
		s.Require().NoError(err)

		events, err := outboxRepo.PickEvents(s.ctx, 10, 30)
		s.Require().NoError(err)

		var found *models.OutboxEvent
		for _, e := range events {
			if e.Id == eventID {
				found = &e
				break
			}
		}

		s.Require().NotNil(found, "created outbox event should be found in picked events")
		s.Equal(eventID, found.Id)
		s.NotNil(found.Headers)
		s.Empty(found.Headers)

		// Clean up
		err = outboxRepo.MarkAsCompleted(s.ctx, eventID)
		s.Require().NoError(err)
	})
}

func (s *IntegrationTestSuite) TestOutboxSkipsLockedEventsAndRecoversExpiredClaims() {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	repo := pg.NewOutboxRepo(s.dbPool)
	first, second := uuid.New(), uuid.New()
	for i, id := range []uuid.UUID{first, second} {
		s.Require().NoError(repo.CreateEvent(ctx, &models.CreateOutboxEventParams{
			Id: id, AggregateID: uuid.New(), EventType: models.OutboxEventSubmissionCreated, Payload: []byte(`{}`),
		}))
		_, err := s.dbPool.Exec(ctx, "UPDATE outbox_events SET created_at=$2 WHERE id=$1", id, time.Date(2000, 1, 1, 0, i, 0, 0, time.UTC))
		s.Require().NoError(err)
	}
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, "DELETE FROM outbox_events WHERE id = ANY($1)", []uuid.UUID{first, second})
		s.Require().NoError(err)
	}()
	tx, err := s.dbPool.Begin(ctx)
	s.Require().NoError(err)
	defer tx.Rollback(s.ctx)
	locked, err := repo.WithTx(tx).PickEvents(ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(locked, 1)
	s.Equal(first, locked[0].Id)

	// A second connection must make progress while the first claim is uncommitted.
	claimCtx, cancelClaim := context.WithTimeout(ctx, 2*time.Second)
	defer cancelClaim()
	claimed, err := repo.PickEvents(claimCtx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(claimed, 1)
	s.Equal(second, claimed[0].Id)
	s.NotEqual(locked[0].Id, claimed[0].Id)
	s.Require().NoError(tx.Rollback(ctx))

	retry, err := repo.PickEvents(ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(retry, 1)
	s.Equal(first, retry[0].Id)
	s.Require().NoError(repo.MarkAsCompleted(ctx, first))
	_, err = s.dbPool.Exec(ctx, "UPDATE outbox_events SET deadline_at=NOW()-INTERVAL '1 second' WHERE id=$1", second)
	s.Require().NoError(err)
	expired, err := repo.PickEvents(ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(expired, 1)
	s.Equal(second, expired[0].Id)
	s.Require().NotNil(expired[0].DeadlineAt)
	s.True(expired[0].DeadlineAt.After(time.Now()))
}
