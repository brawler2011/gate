package pubsub

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/nats-io/nats.go/jetstream"
)

type ContestsSub = StreamSubscriber

func NewContestsSub(
	ctx context.Context,
	js jetstream.JetStream,
	dispatcher interfaces.EventDispatcher,
) (*ContestsSub, error) {
	return NewStreamSubscriber(
		ctx,
		js,
		dispatcher,
		"CONTESTS",
		"contest_events_consumer",
		"contest.*",
		"contest_events_subscriber",
	)
}
