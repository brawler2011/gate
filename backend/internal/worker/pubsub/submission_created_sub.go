package pubsub

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/nats-io/nats.go/jetstream"
)

type SubmissionsSub = StreamSubscriber

func NewSubmissionsSub(
	ctx context.Context,
	js jetstream.JetStream,
	dispatcher interfaces.EventDispatcher,
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
