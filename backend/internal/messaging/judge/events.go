package judgeworker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	messaging "github.com/brawler2011/gate/backend/internal/messaging"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// EventPublisher publishes best-effort judging progress. Terminal events use outbox.
type EventPublisher struct{ js jetstream.JetStream }

func NewEventPublisher(js jetstream.JetStream) *EventPublisher { return &EventPublisher{js: js} }
func (p *EventPublisher) publish(ctx context.Context, subject string, id uuid.UUID, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode judging progress: %w", err)
	}
	ctx = queuecontext.With(ctx, queuecontext.Metadata{EventID: uuid.New(), QueuedAt: time.Now(), AggregateID: id})
	_, err = p.js.PublishMsg(ctx, &nats.Msg{Subject: subject, Data: payload, Header: messaging.Headers(ctx)})
	if err != nil {
		return fmt.Errorf("publish judging progress: %w", err)
	}
	return nil
}
func (p *EventPublisher) PublishQueued(ctx context.Context, id uuid.UUID, meta submissionsmodel.SubmissionEventMeta) error {
	return p.publish(ctx, submissionsmodel.SubmissionEventQueued, id, submissionsmodel.SubmissionQueuedEvent{Id: id, SubmissionEventMeta: meta})
}
func (p *EventPublisher) PublishCompilingStarted(ctx context.Context, id uuid.UUID, meta submissionsmodel.SubmissionEventMeta) error {
	return p.publish(ctx, submissionsmodel.SubmissionEventCompilingStarted, id, submissionsmodel.SubmissionCompilingStartedEvent{Id: id, SubmissionEventMeta: meta})
}
func (p *EventPublisher) PublishTestingStarted(ctx context.Context, id uuid.UUID, meta submissionsmodel.SubmissionEventMeta) error {
	return p.publish(ctx, submissionsmodel.SubmissionEventTestingStarted, id, submissionsmodel.SubmissionTestingStartedEvent{Id: id, SubmissionEventMeta: meta})
}
func (p *EventPublisher) PublishTestStarted(ctx context.Context, id uuid.UUID, number int32, meta submissionsmodel.SubmissionEventMeta) error {
	return p.publish(ctx, submissionsmodel.SubmissionEventTestStarted, id, submissionsmodel.SubmissionTestStartedEvent{Id: id, Number: number, SubmissionEventMeta: meta})
}
