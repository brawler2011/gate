//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	pubsub "github.com/brawler2011/gate/backend/internal/messaging/pubsub"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
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
	repo := pgstore.NewOutboxRepo(s.dbPool)
	aggregate := uuid.New()
	for range 2 {
		s.Require().NoError(repo.Record(s.ctx, eventdata.Event{AggregateID: aggregate, Type: eventdata.OutboxEventSubmissionCreated, Payload: map[string]string{"value": "test"}}))
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
	inbox := pgstore.NewInbox(s.dbPool)
	input := uuid.New()
	output := eventdata.Event{AggregateID: uuid.New(), Type: eventdata.OutboxEventSubmissionCreated, Payload: map[string]string{"effect": "once"}}
	failure := errors.New("injected effect failure")
	err := inbox.Within(s.ctx, "consumer", input, func(writer eventdata.Writer) error {
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
	effect := func(writer eventdata.Writer) error { calls++; return writer.Record(s.ctx, output) }
	s.Require().NoError(inbox.Within(s.ctx, "consumer", input, effect))
	// The first transaction committed, but the broker Ack was lost.
	s.Require().NoError(inbox.Within(s.ctx, "consumer", input, effect))
	s.Equal(1, calls)
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events").Scan(&count))
	s.Equal(1, count)
	s.PanicsWithValue(failure, func() {
		_ = inbox.Within(s.ctx, "consumer", uuid.New(), func(eventdata.Writer) error { panic(failure) })
	})
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM inbox_receipts").Scan(&count))
	s.Equal(1, count)
}

func (s *IntegrationTestSuite) TestOutboxRetriesSameMessageAfterPubAckCrash() {
	s.resetDeliveryTables()
	js, _ := newNATSJetStream(s.T())
	s.Require().NoError(pkg.EnsureSubmissionsStream(s.ctx, js))
	repo := pgstore.NewOutboxRepo(s.dbPool)
	s.Require().NoError(repo.Record(s.ctx, eventdata.Event{AggregateID: uuid.New(), Type: eventdata.OutboxEventSubmissionCreated, Payload: submissionsmodel.SubmissionCreatedEvent{Id: uuid.New()}}))
	publisher := pubsub.NewSubmissionCreatedPublisher(js)
	publish := func(event eventdata.OutboxEvent) {
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

func (s *IntegrationTestSuite) TestOutboxBoundsRetryAndRetainsPermanentFailures() {
	s.resetDeliveryTables()
	repo := pgstore.NewOutboxRepo(s.dbPool)
	aggregate := uuid.New()
	s.Require().NoError(repo.Record(s.ctx, eventdata.Event{AggregateID: aggregate, Type: "test.retry", Payload: struct{}{}}))
	_, err := s.dbPool.Exec(s.ctx, "UPDATE outbox_events SET retry_count=100 WHERE aggregate_id=$1", aggregate)
	s.Require().NoError(err)
	picked, err := repo.PickEvents(s.ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(picked, 1)
	s.Require().NoError(repo.MarkAsFailed(s.ctx, picked[0].Id, picked[0].LeaseToken, "temporary", false))
	var delay float64
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT EXTRACT(EPOCH FROM next_attempt_at-processed_at)::float8 FROM outbox_events WHERE id=$1", picked[0].Id).Scan(&delay))
	s.Equal(float64(300), delay, "large retry counts must keep a bounded delay")
	_, err = s.dbPool.Exec(s.ctx, "UPDATE outbox_events SET next_attempt_at=now()-interval '1 second' WHERE id=$1", picked[0].Id)
	s.Require().NoError(err)
	picked, err = repo.PickEvents(s.ctx, 1, 30)
	s.Require().NoError(err)
	s.Require().Len(picked, 1)
	s.Require().NoError(repo.MarkAsFailed(s.ctx, picked[0].Id, picked[0].LeaseToken, "permanent", true))
	_, err = s.dbPool.Exec(s.ctx, "UPDATE outbox_events SET processed_at=now()-interval '30 days' WHERE id=$1", picked[0].Id)
	s.Require().NoError(err)
	s.Require().NoError(repo.Record(s.ctx, eventdata.Event{AggregateID: aggregate, Type: "test.later", Payload: struct{}{}}))
	s.Require().NoError(repo.DeleteOldEvents(s.ctx, 14))
	blocked, err := repo.PickEvents(s.ctx, 10, 30)
	s.Require().NoError(err)
	s.Empty(blocked, "retained permanent failures block later aggregate versions")
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events WHERE id=$1 AND status='failed' AND error_message='permanent'", picked[0].Id).Scan(&count))
	s.Equal(1, count)
}

func (s *IntegrationTestSuite) TestInboxConcurrentRedeliveriesApplyOncePerConsumer() {
	s.resetDeliveryTables()
	inbox := pgstore.NewInbox(s.dbPool)
	eventID := uuid.New()
	var calls atomic.Int32
	output := eventdata.Event{AggregateID: uuid.New(), Type: "test.effect", Payload: struct{}{}}
	effect := func(writer eventdata.Writer) error {
		calls.Add(1)
		return writer.Record(s.ctx, output)
	}
	start := make(chan struct{})
	results := make(chan error, 8)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			<-start
			results <- inbox.Within(s.ctx, "concurrent", eventID, effect)
		})
	}
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		s.Require().NoError(err)
	}
	s.EqualValues(1, calls.Load())
	s.Require().NoError(inbox.Within(s.ctx, "other-consumer", eventID, effect))
	s.EqualValues(2, calls.Load(), "receipts are scoped to the consumer")
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM inbox_receipts WHERE event_id=$1", eventID).Scan(&count))
	s.Equal(2, count)
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1", output.AggregateID).Scan(&count))
	s.Equal(2, count)
}
