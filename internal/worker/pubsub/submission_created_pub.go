package pubsub

import (
	"context"
	"fmt"

	"github.com/gate149/core/internal/domain/interfaces"
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
	_, err := h.natsJetStream.PublishMsg(ctx, &nats.Msg{
		Subject: subject,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish msg: %w", err)
	}

	return nil
}
