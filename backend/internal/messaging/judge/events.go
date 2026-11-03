package judgeworker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
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
func (ep *EventPublisher) PublishQueued(ctx context.Context, submissionID uuid.UUID, meta submissionsmodel.SubmissionEventMeta) error {
	event := submissionsmodel.SubmissionQueuedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: submissionsmodel.SubmissionEventQueued,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish queued event: %w", err)
	}

	return nil
}

// PublishCompilingStarted publishes submission.compiling_started event
func (ep *EventPublisher) PublishCompilingStarted(ctx context.Context, submissionID uuid.UUID, meta submissionsmodel.SubmissionEventMeta) error {
	event := submissionsmodel.SubmissionCompilingStartedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: submissionsmodel.SubmissionEventCompilingStarted,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish compiling started event: %w", err)
	}

	return nil
}

// PublishTestingStarted publishes submission.testing_started event
func (ep *EventPublisher) PublishTestingStarted(ctx context.Context, submissionID uuid.UUID, meta submissionsmodel.SubmissionEventMeta) error {
	event := submissionsmodel.SubmissionTestingStartedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: submissionsmodel.SubmissionEventTestingStarted,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish testing started event: %w", err)
	}

	return nil
}

// PublishTestStarted publishes submission.test_started event
func (ep *EventPublisher) PublishTestStarted(ctx context.Context, submissionID uuid.UUID, testNumber int32, meta submissionsmodel.SubmissionEventMeta) error {
	event := submissionsmodel.SubmissionTestStartedEvent{
		SubmissionEventMeta: meta,
		Id:                  submissionID,
		Number:              testNumber,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: submissionsmodel.SubmissionEventTestStarted,
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
	state submissionsmodel.State,
	score int32,
	timeStat int32,
	memoryStat int32,
	penalty int32,
	failedTest *int32,
	previousState submissionsmodel.State,
	meta submissionsmodel.SubmissionEventMeta,
) error {
	meta.UpdatedAt = time.Now()
	event := submissionsmodel.SubmissionCompletedEvent{
		PreviousState:       previousState,
		SubmissionEventMeta: meta,
		Id:                  submissionID,
		State:               state,
		Score:               score,
		Penalty:             penalty,
		TimeStat:            timeStat,
		MemoryStat:          memoryStat,
		FailedTest:          failedTest,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = ep.js.PublishMsg(ctx, &nats.Msg{
		Subject: submissionsmodel.SubmissionEventCompleted,
		Data:    payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish completed event: %w", err)
	}

	return nil
}
