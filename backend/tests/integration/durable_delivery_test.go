//go:build integration

package integration

import (
	"context"
	"errors"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/brawler2011/gate/backend/internal/worker/pubsub"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) resetDeliveryTables() {
	_, err := s.dbPool.Exec(s.ctx, "TRUNCATE outbox_events,event_aggregates,inbox_receipts")
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TestOutboxFencesLeasesAndOrdersAggregate() {
	s.resetDeliveryTables()
	repo := pg.NewOutboxRepo(s.dbPool)
	aggregate := uuid.New()
	for range 2 {
		s.Require().NoError(repo.Record(s.ctx, events.Event{AggregateID: aggregate, Type: models.OutboxEventSubmissionCreated, Payload: map[string]string{"value": "test"}}))
	}
	first, err := repo.PickEvents(s.ctx, 10, 30)
	s.Require().NoError(err)
	s.Require().Len(first, 1)
	s.EqualValues(1, first[0].AggregateVersion)
	s.NotEqual(uuid.Nil, first[0].LeaseToken)
	blocked, err := repo.PickEvents(s.ctx, 10, 30)
	s.Require().NoError(err)
	s.Empty(blocked)
	_, err = s.dbPool.Exec(s.ctx, "UPDATE outbox_events SET deadline_at=now()-interval '1 second' WHERE id=$1", first[0].Id)
	s.Require().NoError(err)
	replacement, err := repo.PickEvents(s.ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(replacement, 1)
	s.Equal(first[0].Id, replacement[0].Id)
	s.NotEqual(first[0].LeaseToken, replacement[0].LeaseToken)
	s.ErrorIs(repo.MarkAsCompleted(s.ctx, first[0].Id, first[0].LeaseToken), pkg.ErrConflict)
	s.ErrorIs(repo.MarkAsFailed(s.ctx, first[0].Id, first[0].LeaseToken, "late", false), pkg.ErrConflict)
	s.Require().NoError(repo.MarkAsFailed(s.ctx, replacement[0].Id, replacement[0].LeaseToken, "temporary", false))
	blocked, err = repo.PickEvents(s.ctx, 10, 30)
	s.Require().NoError(err)
	s.Empty(blocked, "retry delay cannot let the next aggregate version overtake")
	_, err = s.dbPool.Exec(s.ctx, "UPDATE outbox_events SET next_attempt_at=now()-interval '1 second' WHERE id=$1", first[0].Id)
	s.Require().NoError(err)
	retry, err := repo.PickEvents(s.ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(retry, 1)
	s.Equal(first[0].Id, retry[0].Id)
	s.Require().NoError(repo.MarkAsCompleted(s.ctx, retry[0].Id, retry[0].LeaseToken))
	next, err := repo.PickEvents(s.ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(next, 1)
	s.EqualValues(2, next[0].AggregateVersion)
	s.Require().NoError(repo.MarkAsFailed(s.ctx, next[0].Id, next[0].LeaseToken, "permanent", true))
	var status string
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT status FROM outbox_events WHERE id=$1", next[0].Id).Scan(&status))
	s.Equal("failed", status)
}

func (s *IntegrationTestSuite) TestInboxCommitsEffectsOnceAndRollsBackFailures() {
	s.resetDeliveryTables()
	inbox := pg.NewInbox(s.dbPool)
	input := uuid.New()
	output := events.Event{AggregateID: uuid.New(), Type: models.OutboxEventSubmissionCreated, Payload: map[string]string{"effect": "once"}}
	failure := errors.New("injected effect failure")
	err := inbox.Within(s.ctx, "consumer", input, func(writer events.Writer) error {
		if err := writer.Record(s.ctx, output); err != nil {
			return err
		}
		return failure
	})
	s.Same(failure, err)
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM inbox_receipts").Scan(&count))
	s.Zero(count)
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events").Scan(&count))
	s.Zero(count)
	calls := 0
	effect := func(writer events.Writer) error { calls++; return writer.Record(s.ctx, output) }
	s.Require().NoError(inbox.Within(s.ctx, "consumer", input, effect))
	// The first transaction committed, but the broker Ack was lost.
	s.Require().NoError(inbox.Within(s.ctx, "consumer", input, effect))
	s.Equal(1, calls)
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events").Scan(&count))
	s.Equal(1, count)
	s.PanicsWithValue(failure, func() { _ = inbox.Within(s.ctx, "consumer", uuid.New(), func(events.Writer) error { panic(failure) }) })
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM inbox_receipts").Scan(&count))
	s.Equal(1, count)
}

func (s *IntegrationTestSuite) TestOutboxRetriesSameMessageAfterPubAckCrash() {
	s.resetDeliveryTables()
	js, _ := newNATSJetStream(s.T())
	s.Require().NoError(pkg.EnsureSubmissionsStream(s.ctx, js))
	repo := pg.NewOutboxRepo(s.dbPool)
	s.Require().NoError(repo.Record(s.ctx, events.Event{AggregateID: uuid.New(), Type: models.OutboxEventSubmissionCreated, Payload: models.SubmissionCreatedEvent{Id: uuid.New()}}))
	publisher := pubsub.NewSubmissionCreatedPublisher(js)
	publish := func(event models.OutboxEvent) {
		ctx := queuecontext.With(context.Background(), queuecontext.Metadata{EventID: event.Id, QueuedAt: event.CreatedAt})
		s.Require().NoError(publisher.Handle(ctx, event.EventType, event.Payload))
	}
	first, err := repo.PickEvents(s.ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(first, 1)
	publish(first[0])
	// Simulate process death after PubAck and before the DB publication mark.
	_, err = s.dbPool.Exec(s.ctx, "UPDATE outbox_events SET deadline_at=now()-interval '1 second' WHERE id=$1", first[0].Id)
	s.Require().NoError(err)
	retry, err := repo.PickEvents(s.ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(retry, 1)
	publish(retry[0])
	s.Require().NoError(repo.MarkAsCompleted(s.ctx, retry[0].Id, retry[0].LeaseToken))
	stream, err := js.Stream(s.ctx, pkg.SubmissionsStreamName)
	s.Require().NoError(err)
	info, err := stream.Info(s.ctx)
	s.Require().NoError(err)
	s.EqualValues(1, info.State.Msgs, "republication must keep the stable Nats-Msg-Id")
}
