package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	messaging "github.com/brawler2011/gate/backend/internal/messaging"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type SubmissionCreatedPublisher struct {
	natsJetStream jetstream.JetStream
}

func NewSubmissionCreatedPublisher(natsJetStream jetstream.JetStream) EventHandler {
	return &SubmissionCreatedPublisher{
		natsJetStream: natsJetStream,
	}
}

const subject = "submissions.created"

func (h *SubmissionCreatedPublisher) Handle(ctx context.Context, eventType string, payload []byte) error {
	if eventType == eventdata.OutboxEventSubmissionCreated {
		eventType = subject
	}
	msg := &nats.Msg{
		Subject: eventType,
		Data:    payload,
	}
	msg.Header = messaging.Headers(ctx)
	meta := queuecontext.Get(ctx)

	_, err := h.natsJetStream.PublishMsg(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to publish msg: %w", err)
	}

	var event submissionsmodel.SubmissionCreatedEvent
	if json.Unmarshal(payload, &event) == nil {
		logger := slog.Default().With("submission_id", event.Id, "request_id", requestid.Get(ctx))
		if meta.EventID != uuid.Nil {
			logger = logger.With("event_id", meta.EventID)
		}
		logger.Info("submission_queued", "event", "submission_queued")
	}
	return nil
}
