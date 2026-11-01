//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	corev1 "github.com/brawler2011/contracts/core/v1"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/brawler2011/gate/backend/internal/worker/judge"
	"github.com/brawler2011/gate/backend/internal/worker/outbox"
	"github.com/brawler2011/gate/backend/internal/worker/pubsub"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type diagnosticJudgingCall struct {
	id        uuid.UUID
	requestID string
}
type diagnosticJudge struct{ calls chan diagnosticJudgingCall }

func (j diagnosticJudge) JudgeSubmission(ctx context.Context, id uuid.UUID) error {
	j.calls <- diagnosticJudgingCall{id, requestid.Get(ctx)}
	return nil
}

// checkDiagnosticDelivery follows a real HTTP-created event through PostgreSQL,
// the production outbox worker, JetStream and the production judge worker.
// Only the expensive sandbox judging usecase is replaced at the worker boundary.
func (s *IntegrationTestSuite) checkDiagnosticDelivery(submissionID, adminID uuid.UUID, orgLogin string) {
	requestID := s.transport.lastResponse.Header.Get(requestid.Header)
	s.Require().True(requestid.Valid(requestID))
	var eventID uuid.UUID
	var headersJSON []byte
	var createdAt time.Time
	err := s.dbPool.QueryRow(s.ctx, "SELECT id, headers, created_at FROM outbox_events WHERE aggregate_id=$1 AND event_type=$2", submissionID, models.OutboxEventSubmissionCreated).Scan(&eventID, &headersJSON, &createdAt)
	s.Require().NoError(err)
	var headers map[string]string
	s.Require().NoError(json.Unmarshal(headersJSON, &headers))
	s.Equal(requestID, headers[requestid.Header])

	container, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: "nats:2.10-alpine", ExposedPorts: []string{"4222/tcp"}, Cmd: []string{"-js"}, WaitingFor: wait.ForListeningPort("4222/tcp"),
	}, Started: true})
	s.Require().NoError(err)
	defer func() { s.NoError(container.Terminate(context.Background())) }()
	host, err := container.Host(s.ctx)
	s.Require().NoError(err)
	port, err := container.MappedPort(s.ctx, "4222/tcp")
	s.Require().NoError(err)
	conn, err := nats.Connect(fmt.Sprintf("nats://%s:%s", host, port.Port()))
	s.Require().NoError(err)
	defer conn.Close()
	js, err := jetstream.New(conn)
	s.Require().NoError(err)
	s.Require().NoError(pkg.EnsureSubmissionsStream(s.ctx, js))
	s.Require().NoError(pkg.EnsureContestsStream(s.ctx, js))
	tap, err := js.CreateOrUpdateConsumer(s.ctx, pkg.SubmissionsStreamName, jetstream.ConsumerConfig{Durable: "diagnostic_tap", FilterSubject: "submissions.created", AckPolicy: jetstream.AckExplicitPolicy})
	s.Require().NoError(err)
	calls := make(chan diagnosticJudgingCall, 10)
	worker, err := judge.NewJudgeWorker(s.ctx, js, diagnosticJudge{calls}, 1)
	s.Require().NoError(err)
	dispatcher := outbox.NewEventDispatcher()
	dispatcher.Register(models.OutboxEventSubmissionCreated, pubsub.NewSubmissionCreatedPublisher(js))
	for _, kind := range []string{models.OutboxEventContestAnnouncementCreated, models.OutboxEventContestAnnouncementDeleted, models.OutboxEventContestClarificationCreated, models.OutboxEventContestClarificationAnswered} {
		dispatcher.Register(kind, pubsub.NewContestEventsPublisher(js))
	}
	outboxWorker := outbox.NewOutboxWorker(dispatcher, pg.NewOutboxRepo(s.dbPool))
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	defer func() { cancel(); <-stopped }()
	go func() { _ = worker.Start(ctx); close(stopped) }()
	outboxStopped := make(chan struct{})
	defer func() { cancel(); <-outboxStopped }()
	go func() { outboxWorker.Start(ctx); close(outboxStopped) }()
	select {
	case call := <-calls:
		s.Equal(submissionID, call.id)
		s.Equal(requestID, call.requestID)
	case <-time.After(15 * time.Second):
		s.FailNow("HTTP-created submission did not reach judge worker")
	}
	msg, err := tap.Next(jetstream.FetchMaxWait(5 * time.Second))
	s.Require().NoError(err)
	s.Equal(requestID, msg.Headers().Get(requestid.Header))
	s.Equal(eventID.String(), msg.Headers().Get(queuecontext.EventIDHeader))
	queuedAt, err := time.Parse(time.RFC3339Nano, msg.Headers().Get(queuecontext.QueuedAtHeader))
	s.Require().NoError(err)
	s.True(createdAt.Equal(queuedAt))
	var createdEvent models.SubmissionCreatedEvent
	s.Require().NoError(json.Unmarshal(msg.Data(), &createdEvent))
	s.True(createdEvent.IsNew)
	s.Nil(createdEvent.PreviousState)
	s.Require().NoError(msg.Ack())
	s.Eventually(func() bool {
		var status string
		if err := s.dbPool.QueryRow(s.ctx, "SELECT status FROM outbox_events WHERE id=$1", eventID).Scan(&status); err != nil {
			return false
		}
		return status == models.OutboxEventStatusCompleted
	}, 5*time.Second, 20*time.Millisecond)
	// Make the submission old, then request a new rejudge through HTTP.
	_, err = s.dbPool.Exec(s.ctx, "UPDATE submissions SET created_at=NOW()-INTERVAL '1 day', state=200 WHERE id=$1", submissionID)
	s.Require().NoError(err)
	err = s.client.RejudgeSubmission(withTestUser(s.ctx, adminID), corev1.RejudgeSubmissionParams{OrgLogin: orgLogin, ContestLogin: "submission-contest", SubmissionID: submissionID})
	s.Require().NoError(err)
	rejudgeRequestID := s.transport.lastResponse.Header.Get(requestid.Header)
	var rejudgeEventID uuid.UUID
	var rejudgeCreatedAt time.Time
	err = s.dbPool.QueryRow(s.ctx, "SELECT id, created_at FROM outbox_events WHERE aggregate_id=$1 AND id<>$2 ORDER BY created_at DESC LIMIT 1", submissionID, eventID).Scan(&rejudgeEventID, &rejudgeCreatedAt)
	s.Require().NoError(err)
	s.NotEqual(eventID, rejudgeEventID)
	select {
	case call := <-calls:
		s.Equal(submissionID, call.id)
		s.Equal(rejudgeRequestID, call.requestID)
	case <-time.After(15 * time.Second):
		s.FailNow("rejudge did not reach worker")
	}
	rejudgeMsg, err := tap.Next(jetstream.FetchMaxWait(5 * time.Second))
	s.Require().NoError(err)
	s.Equal(rejudgeEventID.String(), rejudgeMsg.Headers().Get(queuecontext.EventIDHeader))
	rejudgeQueuedAt, err := time.Parse(time.RFC3339Nano, rejudgeMsg.Headers().Get(queuecontext.QueuedAtHeader))
	s.Require().NoError(err)
	s.True(rejudgeCreatedAt.Equal(rejudgeQueuedAt))
	s.WithinDuration(time.Now(), rejudgeQueuedAt, 30*time.Second)
	var resetEvent models.SubmissionCreatedEvent
	s.Require().NoError(json.Unmarshal(rejudgeMsg.Data(), &resetEvent))
	s.False(resetEvent.IsNew)
	s.Require().NotNil(resetEvent.PreviousState)
	s.Equal(models.Accepted, *resetEvent.PreviousState)
	s.Require().NoError(rejudgeMsg.Ack())

}
