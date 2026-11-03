package pubsub

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

type SubmissionsSub = StreamSubscriber

func NewSubmissionsSub(
	ctx context.Context,
	js jetstream.JetStream,
	dispatcher Dispatcher,
) (*SubmissionsSub, error) {
	return NewStreamSubscriber(
		ctx,
		js,
		dispatcher,
		"SUBMISSIONS",
		"submissions_consumer",
		"submissions.*",
		"submission_created_subscriber",
	)
}
