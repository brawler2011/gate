package observer

import (
	"encoding/json"
	"fmt"

	observerv1 "github.com/gate149/contracts/observer/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
)

func strPtrIfNotEmpty(v string) *string {
	if v == "" {
		return nil
	}
	copy := v
	return &copy
}

func int32PtrIfNonZero(v int32) *int32 {
	if v == 0 {
		return nil
	}
	copy := v
	return &copy
}

func int32Ptr(v *int32) *int32 {
	if v == nil {
		return nil
	}
	copy := *v
	return &copy
}

func SubmissionsEnvelopeDTO(eventType string, event any) ([]byte, error) {
	msg := observerv1.SubmissionsMessage{
		EventType: observerv1.SubmissionsEventType(eventType),
	}

	switch eventType {
	case models.SubmissionEventCreated:
		e, ok := event.(*models.SubmissionCreatedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		if err := msg.Payload.FromMessageSubmissionCreated(SubmissionCreatedDTO(*e)); err != nil {
			return nil, fmt.Errorf("encode submissions.created payload: %w", err)
		}
	case models.SubmissionEventQueued:
		e, ok := event.(*models.SubmissionQueuedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		if err := msg.Payload.FromMessageSubmissionQueued(SubmissionQueuedDTO(*e)); err != nil {
			return nil, fmt.Errorf("encode submissions.queued payload: %w", err)
		}
	case models.SubmissionEventCompilingStarted:
		e, ok := event.(*models.SubmissionCompilingStartedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		if err := msg.Payload.FromMessageSubmissionCompilingStarted(SubmissionCompilingStartedDTO(*e)); err != nil {
			return nil, fmt.Errorf("encode submissions.compiling_started payload: %w", err)
		}
	case models.SubmissionEventTestingStarted:
		e, ok := event.(*models.SubmissionTestingStartedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		if err := msg.Payload.FromMessageSubmissionTestingStarted(SubmissionTestingStartedDTO(*e)); err != nil {
			return nil, fmt.Errorf("encode submissions.testing_started payload: %w", err)
		}
	case models.SubmissionEventTestStarted:
		e, ok := event.(*models.SubmissionTestStartedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		if err := msg.Payload.FromMessageSubmissionTestStarted(SubmissionTestStartedDTO(*e)); err != nil {
			return nil, fmt.Errorf("encode submissions.test_started payload: %w", err)
		}
	case models.SubmissionEventCompleted:
		e, ok := event.(*models.SubmissionCompletedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		if err := msg.Payload.FromMessageSubmissionCompleted(SubmissionCompletedDTO(*e)); err != nil {
			return nil, fmt.Errorf("encode submissions.completed payload: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal submissions message: %w", err)
	}

	return data, nil
}

func SubmissionCreatedDTO(e models.SubmissionCreatedEvent) observerv1.MessageSubmissionCreated {
	return observerv1.MessageSubmissionCreated{
		ContestId:    e.ContestId,
		ContestTitle: strPtrIfNotEmpty(e.ContestTitle),
		CreatedAt:    &e.CreatedAt,
		Id:           e.Id,
		Language:     int32PtrIfNonZero(e.Language),
		Position:     int32Ptr(e.Position),
		ProblemId:    e.ProblemId,
		ProblemTitle: strPtrIfNotEmpty(e.ProblemTitle),
		Source:       e.Source,
		State:        e.State,
		UserId:       e.UserId,
		Username:     strPtrIfNotEmpty(e.Username),
	}
}

func SubmissionQueuedDTO(e models.SubmissionQueuedEvent) observerv1.MessageSubmissionQueued {
	return observerv1.MessageSubmissionQueued{
		ContestId:    e.ContestId,
		ContestTitle: strPtrIfNotEmpty(e.ContestTitle),
		CreatedAt:    &e.CreatedAt,
		Id:           e.Id,
		Language:     int32PtrIfNonZero(e.Language),
		Position:     int32Ptr(e.Position),
		ProblemId:    e.ProblemId,
		ProblemTitle: strPtrIfNotEmpty(e.ProblemTitle),
		UserId:       e.UserId,
		Username:     strPtrIfNotEmpty(e.Username),
	}
}

func SubmissionCompilingStartedDTO(e models.SubmissionCompilingStartedEvent) observerv1.MessageSubmissionCompilingStarted {
	return observerv1.MessageSubmissionCompilingStarted{
		ContestId:    e.ContestId,
		ContestTitle: strPtrIfNotEmpty(e.ContestTitle),
		CreatedAt:    &e.CreatedAt,
		Id:           e.Id,
		Language:     int32PtrIfNonZero(e.Language),
		Position:     int32Ptr(e.Position),
		ProblemId:    e.ProblemId,
		ProblemTitle: strPtrIfNotEmpty(e.ProblemTitle),
		UserId:       e.UserId,
		Username:     strPtrIfNotEmpty(e.Username),
	}
}

func SubmissionTestingStartedDTO(e models.SubmissionTestingStartedEvent) observerv1.MessageSubmissionTestingStarted {
	return observerv1.MessageSubmissionTestingStarted{
		ContestId:    e.ContestId,
		ContestTitle: strPtrIfNotEmpty(e.ContestTitle),
		CreatedAt:    &e.CreatedAt,
		Id:           e.Id,
		Language:     int32PtrIfNonZero(e.Language),
		Position:     int32Ptr(e.Position),
		ProblemId:    e.ProblemId,
		ProblemTitle: strPtrIfNotEmpty(e.ProblemTitle),
		UserId:       e.UserId,
		Username:     strPtrIfNotEmpty(e.Username),
	}
}

func SubmissionTestStartedDTO(e models.SubmissionTestStartedEvent) observerv1.MessageSubmissionTestStarted {
	return observerv1.MessageSubmissionTestStarted{
		ContestId:    e.ContestId,
		ContestTitle: strPtrIfNotEmpty(e.ContestTitle),
		CreatedAt:    &e.CreatedAt,
		Id:           e.Id,
		Language:     int32PtrIfNonZero(e.Language),
		Number:       e.Number,
		Position:     int32Ptr(e.Position),
		ProblemId:    e.ProblemId,
		ProblemTitle: strPtrIfNotEmpty(e.ProblemTitle),
		UserId:       e.UserId,
		Username:     strPtrIfNotEmpty(e.Username),
	}
}

func SubmissionCompletedDTO(e models.SubmissionCompletedEvent) observerv1.MessageSubmissionCompleted {
	return observerv1.MessageSubmissionCompleted{
		ContestId:    e.ContestId,
		ContestTitle: strPtrIfNotEmpty(e.ContestTitle),
		CreatedAt:    &e.CreatedAt,
		Id:           e.Id,
		Language:     int32PtrIfNonZero(e.Language),
		MemoryStat:   e.MemoryStat,
		Penalty:      e.Penalty,
		Position:     int32Ptr(e.Position),
		ProblemId:    e.ProblemId,
		ProblemTitle: strPtrIfNotEmpty(e.ProblemTitle),
		Score:        e.Score,
		State:        e.State,
		TimeStat:     e.TimeStat,
		UserId:       e.UserId,
		Username:     strPtrIfNotEmpty(e.Username),
	}
}
