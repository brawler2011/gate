//go:build integration
// +build integration

package integration

import (
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
			"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			"tracestate":  "rojo=1",
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
		s.Equal(headers["traceparent"], found.Headers["traceparent"])
		s.Equal(headers["tracestate"], found.Headers["tracestate"])

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
