package pubsub

import (
	"context"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/pkg/telemetry"
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
	// Propagate the distributed trace context into NATS message headers
	// so the Judge Worker can extract and continue the trace chain.
	telemetry.InjectNATSMsg(ctx, msg)

	_, err := h.natsJetStream.PublishMsg(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to publish msg: %w", err)
	}

	return nil
}
