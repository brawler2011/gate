package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/nats-io/nats.go/jetstream"
)

type SubmissionsSub struct {
	js         jetstream.JetStream
	dispatcher interfaces.EventDispatcher
	consumer   jetstream.Consumer
	logger     *slog.Logger
}

func NewSubmissionsSub(
	ctx context.Context,
	js jetstream.JetStream,
	dispatcher interfaces.EventDispatcher,
) (*SubmissionsSub, error) {
	const (
		streamName    = "SUBMISSIONS"
		durableName   = "submissions_consumer"
		filterSubject = "submissions.*"
	)

	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create nats consumer: %w", err)
	}

	return &SubmissionsSub{
		js:         js,
		dispatcher: dispatcher,
		consumer:   consumer,
		logger:     slog.Default().With("component", "submission_created_subscriber"),
	}, nil
}

func (s *SubmissionsSub) Start(ctx context.Context) error {
	s.logger.Info("starting subscriber")

	consumeCtx, err := s.consumer.Consume(s.handleMessage(ctx), jetstream.PullMaxMessages(1))
	if err != nil {
		return fmt.Errorf("failed to start consume: %w", err)
	}

	select {
	case <-ctx.Done():
		s.logger.Info("stopping subscriber")
		consumeCtx.Stop()
		<-consumeCtx.Closed()
		return ctx.Err()
	case <-consumeCtx.Closed():
		s.logger.Error("consumer closed unexpectedly")
		return fmt.Errorf("consumer closed")
	}
}

func (s *SubmissionsSub) handleMessage(parentCtx context.Context) jetstream.MessageHandler {
	return func(msg jetstream.Msg) {
		const handlerTimeout = 30 * time.Second

		ctx, cancel := context.WithTimeout(context.WithoutCancel(parentCtx), handlerTimeout)
		defer cancel()

		meta, err := msg.Metadata()
		if err != nil {
			s.logger.Error("failed to get message metadata", "error", err, "subject", msg.Subject())
			if err := msg.Nak(); err != nil {
				s.logger.Error("failed to nak message", "error", err)
			}
		}

		// Very important to pass sequence info
		ctx = context.WithValue(ctx, "message_stream_sequence", meta.Sequence.Stream)

		if err := s.dispatcher.Dispatch(ctx, msg.Subject(), msg.Data()); err != nil {
			s.logger.Error("failed to handle event", "error", err, "subject", msg.Subject())
			if err := msg.Nak(); err != nil {
				s.logger.Error("failed to nak message", "error", err)
			}
			return
		}

		if err := msg.Ack(); err != nil {
			s.logger.Error("failed to ack message", "error", err)
		}
	}
}
