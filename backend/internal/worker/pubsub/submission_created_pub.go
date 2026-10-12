package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type SubmissionCreatedPublisher struct {
	natsJetStream jetstream.JetStream
}

func NewSubmissionCreatedPublisher(natsJetStream jetstream.JetStream) interfaces.EventHandler {
	return &SubmissionCreatedPublisher{
		natsJetStream: natsJetStream,
	}
}

const subject = "submissions.created"

func (h *SubmissionCreatedPublisher) Handle(ctx context.Context, eventType string, payload []byte) error {
	msg := &nats.Msg{
		Subject: subject,
		Data:    payload,
	}
	msg.Header = nats.Header{}
	if id := requestid.Get(ctx); id != "" {
		msg.Header.Set(requestid.Header, id)
	}

	meta := queuecontext.Get(ctx)
	if meta.EventID != uuid.Nil {
		msg.Header.Set(queuecontext.EventIDHeader, meta.EventID.String())
	}
	if !meta.QueuedAt.IsZero() {
		msg.Header.Set(queuecontext.QueuedAtHeader, meta.QueuedAt.Format(time.RFC3339Nano))
	}
	_, err := h.natsJetStream.PublishMsg(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to publish msg: %w", err)
	}

	var event models.SubmissionCreatedEvent
	if json.Unmarshal(payload, &event) == nil {
		logger := slog.Default().With("submission_id", event.Id, "request_id", requestid.Get(ctx))
		if meta.EventID != uuid.Nil {
			logger = logger.With("event_id", meta.EventID)
		}
		logger.Info("submission_queued", "event", "submission_queued")
	}
	return nil
}
