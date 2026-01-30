package judge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// EventPublisher publishes judging events to NATS
type EventPublisher struct {
	js jetstream.JetStream
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(js jetstream.JetStream) *EventPublisher {
	return &EventPublisher{
		js: js,
	}
}

// PublishQueued publishes submission.queued event
func (ep *EventPublisher) PublishQueued(ctx context.Context, submissionID uuid.UUID, meta models.SubmissionEventMeta) error {
	event := models.SubmissionQueuedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: models.SubmissionEventQueued,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish queued event: %w", err)
	}

	return nil
}

// PublishCompilingStarted publishes submission.compiling_started event
func (ep *EventPublisher) PublishCompilingStarted(ctx context.Context, submissionID uuid.UUID, meta models.SubmissionEventMeta) error {
	event := models.SubmissionCompilingStartedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: models.SubmissionEventCompilingStarted,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish compiling started event: %w", err)
	}

	return nil
}

// PublishTestingStarted publishes submission.testing_started event
func (ep *EventPublisher) PublishTestingStarted(ctx context.Context, submissionID uuid.UUID, meta models.SubmissionEventMeta) error {
	event := models.SubmissionTestingStartedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: models.SubmissionEventTestingStarted,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish testing started event: %w", err)
	}

	return nil
}

// PublishTestStarted publishes submission.test_started event
func (ep *EventPublisher) PublishTestStarted(ctx context.Context, submissionID uuid.UUID, testNumber int32, meta models.SubmissionEventMeta) error {
	event := models.SubmissionTestStartedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
		Number:              testNumber,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: models.SubmissionEventTestStarted,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish test started event: %w", err)
	}

	return nil
}

// PublishCompleted publishes submission.completed event
func (ep *EventPublisher) PublishCompleted(
	ctx context.Context,
	submissionID uuid.UUID,
	state models.State,
	score int32,
	timeStat int32,
	memoryStat int32,
	penalty int32,
	meta models.SubmissionEventMeta,
) error {
	event := models.SubmissionCompletedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
		State:               state,
		Score:               score,
		Penalty:             penalty,
		TimeStat:            timeStat,
		MemoryStat:          memoryStat,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: models.SubmissionEventCompleted,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish completed event: %w", err)
	}

	return nil
}
