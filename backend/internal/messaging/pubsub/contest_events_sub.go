package pubsub

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

type ContestsSub = StreamSubscriber

func NewContestsSub(
	ctx context.Context,
	js jetstream.JetStream,
	dispatcher Dispatcher,
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
