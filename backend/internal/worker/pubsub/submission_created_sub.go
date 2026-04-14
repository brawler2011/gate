package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/nats-io/nats.go/jetstream"
)

type SubmissionsSub struct {
	js         jetstream.JetStream
	dispatcher interfaces.EventDispatcher
	consumer   jetstream.Consumer
	logger     *slog.Logger
	mu         sync.Mutex
	consumeCtx jetstream.ConsumeContext
	cancel     context.CancelFunc
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

	runCtx, cancel := context.WithCancel(ctx)
	consumeCtx, err := s.consumer.Consume(s.handleMessage(runCtx), jetstream.PullMaxMessages(1))
	if err != nil {
		cancel()
		return fmt.Errorf("failed to start consume: %w", err)
	}

	s.mu.Lock()
	s.consumeCtx = consumeCtx
	s.cancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.consumeCtx = nil
		s.cancel = nil
		s.mu.Unlock()
		cancel()
	}()

	if err := runCtx.Err(); err != nil {
		consumeCtx.Stop()
		<-consumeCtx.Closed()
		return err
	}

	select {
	case <-runCtx.Done():
		consumeCtx.Stop()
		<-consumeCtx.Closed()
		return runCtx.Err()
	case <-consumeCtx.Closed():
		if err := runCtx.Err(); err != nil {
			return err
		}
		s.logger.Error("consumer closed unexpectedly")
		return fmt.Errorf("consumer closed")
	}
}

func (s *SubmissionsSub) Stop() {
	s.mu.Lock()
	consumeCtx := s.consumeCtx
	cancel := s.cancel
	s.mu.Unlock()

	if consumeCtx == nil && cancel == nil {
		return
	}

	s.logger.Info("stopping subscriber")
	if cancel != nil {
		cancel()
	}
	if consumeCtx != nil {
		consumeCtx.Stop()
		<-consumeCtx.Closed()
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
			return
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
