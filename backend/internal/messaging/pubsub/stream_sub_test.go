package pubsub

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	messaging "github.com/brawler2011/gate/backend/internal/messaging"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type dispatchFunc func(context.Context, string, []byte) error

func (f dispatchFunc) Dispatch(ctx context.Context, subject string, data []byte) error {
	return f(ctx, subject, data)
}

type deliveryMessage struct {
	jetstream.Msg
	headers        nats.Header
	metadataError  error
	ack, nak, term int
	delay          time.Duration
}

func (m *deliveryMessage) Headers() nats.Header { return m.headers }
func (m *deliveryMessage) Subject() string      { return "contest.announcement_created" }
func (m *deliveryMessage) Data() []byte         { return []byte("payload") }
func (m *deliveryMessage) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{Sequence: jetstream.SequencePair{Stream: 17}}, m.metadataError
}
func (m *deliveryMessage) Ack() error { m.ack++; return nil }
func (m *deliveryMessage) NakWithDelay(delay time.Duration) error {
	m.nak++
	m.delay = delay
	return nil
}
func (m *deliveryMessage) Term() error { m.term++; return nil }

func TestStreamConsumerAcknowledgesOnlySuccessfulHandlers(t *testing.T) {
	failure := errors.New("handler failed")
	for _, tc := range []struct {
		name                   string
		failure, metadataError error
		ack, nak, term, calls  int
	}{
		{name: "committed", ack: 1, calls: 1},
		{name: "transient failure", failure: failure, nak: 1, calls: 1},
		{name: "permanent failure", failure: &eventdata.PermanentError{Cause: failure}, term: 1, calls: 1},
		{name: "metadata failure", metadataError: failure, nak: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event, aggregate := uuid.New(), uuid.New()
			ctx := requestid.With(context.Background(), "request-123")
			ctx = queuecontext.With(ctx, queuecontext.Metadata{EventID: event, AggregateID: aggregate, AggregateVersion: 7, QueuedAt: time.Now().UTC()})
			message := &deliveryMessage{headers: messaging.Headers(ctx), metadataError: tc.metadataError}
			calls := 0
			dispatcher := dispatchFunc(func(ctx context.Context, subject string, data []byte) error {
				calls++
				require.Equal(t, message.Subject(), subject)
				require.Equal(t, message.Data(), data)
				require.Equal(t, event, queuecontext.Get(ctx).EventID)
				require.Equal(t, aggregate, queuecontext.Get(ctx).AggregateID)
				require.EqualValues(t, 7, queuecontext.Get(ctx).AggregateVersion)
				require.Equal(t, "request-123", requestid.Get(ctx))
				require.EqualValues(t, 17, ctx.Value(eventdata.MessageStreamSequenceKey))
				return tc.failure
			})
			subscriber := &StreamSubscriber{dispatcher: dispatcher, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			subscriber.handleMessage(context.Background())(message)
			require.Equal(t, tc.calls, calls)
			require.Equal(t, tc.ack, message.ack)
			require.Equal(t, tc.nak, message.nak)
			require.Equal(t, tc.term, message.term)
			if tc.nak > 0 {
				require.Equal(t, 5*time.Second, message.delay)
			}
		})
	}
}

type infoConsumer struct {
	jetstream.Consumer
	err error
}

func (c infoConsumer) Info(context.Context) (*jetstream.ConsumerInfo, error) {
	return &jetstream.ConsumerInfo{}, c.err
}

type stopRecorder struct {
	jetstream.ConsumeContext
	stopped bool
}

func (c *stopRecorder) Stop() { c.stopped = true }

func TestStreamConsumerStopsOnlyWhenConsumerIsGone(t *testing.T) {
	for _, tc := range []struct {
		name    string
		infoErr error
		stopped bool
	}{
		{name: "consumer deleted", infoErr: jetstream.ErrConsumerNotFound, stopped: true},
		{name: "consumer exists"},
		{name: "server unreachable", infoErr: nats.ErrConnectionClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subscriber := &StreamSubscriber{consumer: infoConsumer{err: tc.infoErr}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			consumeCtx := &stopRecorder{}
			subscriber.handleConsumeError(consumeCtx, nats.ErrNoResponders)
			require.Equal(t, tc.stopped, consumeCtx.stopped)
		})
	}
}
