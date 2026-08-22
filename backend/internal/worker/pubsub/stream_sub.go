package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/nats-io/nats.go/jetstream"
)

type StreamSubscriber struct {
	js         jetstream.JetStream
	dispatcher interfaces.EventDispatcher
	consumer   jetstream.Consumer
	logger     *slog.Logger
	mu         sync.Mutex
	consumeCtx jetstream.ConsumeContext
	cancel     context.CancelFunc
}

func NewStreamSubscriber(
	ctx context.Context,
	js jetstream.JetStream,
	dispatcher interfaces.EventDispatcher,
	streamName string,
	durableName string,
	filterSubject string,
	componentName string,
) (*StreamSubscriber, error) {
	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create nats consumer for %s: %w", streamName, err)
	}

	return &StreamSubscriber{
		js:         js,
		dispatcher: dispatcher,
		consumer:   consumer,
		logger:     slog.Default().With("component", componentName),
	}, nil
}

func (s *StreamSubscriber) Start(ctx context.Context) error {
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

func (s *StreamSubscriber) Stop() {
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

func (s *StreamSubscriber) handleMessage(parentCtx context.Context) jetstream.MessageHandler {
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

		ctx = context.WithValue(ctx, models.MessageStreamSequenceKey, meta.Sequence.Stream)

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
