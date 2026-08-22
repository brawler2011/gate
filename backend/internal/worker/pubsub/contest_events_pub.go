package pubsub

import (
	"context"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/pkg/telemetry"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type ContestEventsPublisher struct {
	natsJetStream jetstream.JetStream
}

func NewContestEventsPublisher(natsJetStream jetstream.JetStream) interfaces.EventHandler {
	return &ContestEventsPublisher{
		natsJetStream: natsJetStream,
	}
}

func (h *ContestEventsPublisher) Handle(ctx context.Context, eventType string, payload []byte) error {
	msg := &nats.Msg{
		Subject: eventType,
		Data:    payload,
	}
	telemetry.InjectNATSMsg(ctx, msg)

	_, err := h.natsJetStream.PublishMsg(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to publish contest event %s: %w", eventType, err)
	}

	return nil
}
