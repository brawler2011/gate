package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/worker/pubsub"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type outboxRepository struct {
	interfaces.OutboxRepo
	completed, failed int
	markErr           error
}

func (r *outboxRepository) MarkAsCompleted(context.Context, uuid.UUID) error {
	r.completed++
	return r.markErr
}
func (r *outboxRepository) MarkAsFailed(context.Context, uuid.UUID, string) error {
	r.failed++
	return r.markErr
}

type eventDispatcher struct{ handler interfaces.EventHandler }

func (d eventDispatcher) Dispatch(ctx context.Context, kind string, data []byte) error {
	return d.handler.Handle(ctx, kind, data)
}

type publishCapture struct {
	jetstream.JetStream
	t     *testing.T
	repo  *outboxRepository
	event models.OutboxEvent
	msg   *nats.Msg
	fail  bool
}

func (p *publishCapture) PublishMsg(ctx context.Context, msg *nats.Msg, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	require.NoError(p.t, ctx.Err())
	require.Zero(p.t, p.repo.completed, "publish must precede marking completed")
	require.Equal(p.t, p.event.Payload, msg.Data)
	p.msg = msg
	if p.fail {
		return nil, errors.New("publish-secret")
	}
	return &jetstream.PubAck{}, nil
}

func TestOutboxPublisherDiagnosticHeadersAndFailure(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)
	for _, fail := range []bool{false, true} {
		for _, id := range []string{"request-123", "", "invalid secret"} {
			for _, markFail := range []bool{false, true} {
				var buf bytes.Buffer
				slog.SetDefault(logging.New(&buf))
				submissionID := uuid.New()
				payload, err := json.Marshal(models.SubmissionCreatedEvent{Id: submissionID})
				require.NoError(t, err)
				// This is a distinct rejudge event; the submission payload's age is irrelevant.
				event := models.OutboxEvent{Id: uuid.New(), AggregateID: submissionID, CreatedAt: time.Now().Add(-time.Second), EventType: models.OutboxEventSubmissionCreated, Payload: payload, Headers: map[string]string{requestid.Header: id}}
				repo := &outboxRepository{}
				if markFail {
					repo.markErr = errors.New("mark-secret")
				}
				capture := &publishCapture{t: t, repo: repo, event: event, fail: fail}
				worker := NewOutboxWorker(eventDispatcher{pubsub.NewSubmissionCreatedPublisher(capture)}, repo)
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				worker.processEvent(ctx, &event)
				require.NotNil(t, capture.msg)
				require.Equal(t, event.Id.String(), capture.msg.Header.Get(queuecontext.EventIDHeader))
				require.Equal(t, event.CreatedAt.Format(time.RFC3339Nano), capture.msg.Header.Get(queuecontext.QueuedAtHeader))
				if requestid.Valid(id) {
					require.Equal(t, id, capture.msg.Header.Get(requestid.Header))
				} else {
					require.Empty(t, capture.msg.Header.Get(requestid.Header))
				}
				require.Empty(t, capture.msg.Header.Get("traceparent"))
				require.NotContains(t, buf.String(), "secret")
				if fail {
					require.Equal(t, 1, repo.failed)
					require.Zero(t, repo.completed)
					require.NotContains(t, buf.String(), "submission_queued")
				} else {
					require.Equal(t, 1, repo.completed)
					require.Zero(t, repo.failed)
					require.Contains(t, buf.String(), "submission_queued")
				}
			}
		}
	}
}
