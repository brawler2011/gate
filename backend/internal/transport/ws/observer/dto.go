package observer

import (
	"encoding/json"
	"fmt"

	observerv1 "github.com/brawler2011/contracts/observer/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
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
	var msg observerv1.SubmissionsMessage

	switch eventType {
	case models.SubmissionEventCreated:
		e, ok := event.(*models.SubmissionCreatedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := SubmissionCreatedDTO(*e)
		msg = observerv1.SubmissionCreatedMessage{
			EventType: new(observerv1.SubmissionCreatedMessageEventType),
			Payload:   &payload,
		}
	case models.SubmissionEventQueued:
		e, ok := event.(*models.SubmissionQueuedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := SubmissionQueuedDTO(*e)
		msg = observerv1.SubmissionQueuedMessage{
			EventType: new(observerv1.SubmissionQueuedMessageEventType),
			Payload:   &payload,
		}
	case models.SubmissionEventCompilingStarted:
		e, ok := event.(*models.SubmissionCompilingStartedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := SubmissionCompilingStartedDTO(*e)
		msg = observerv1.SubmissionCompilingStartedMessage{
			EventType: new(observerv1.SubmissionCompilingStartedMessageEventType),
			Payload:   &payload,
		}
	case models.SubmissionEventTestingStarted:
		e, ok := event.(*models.SubmissionTestingStartedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := SubmissionTestingStartedDTO(*e)
		msg = observerv1.SubmissionTestingStartedMessage{
			EventType: new(observerv1.SubmissionTestingStartedMessageEventType),
			Payload:   &payload,
		}
	case models.SubmissionEventTestStarted:
		e, ok := event.(*models.SubmissionTestStartedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := SubmissionTestStartedDTO(*e)
		msg = observerv1.SubmissionTestStartedMessage{
			EventType: new(observerv1.SubmissionTestStartedMessageEventType),
			Payload:   &payload,
		}
	case models.SubmissionEventCompleted:
		e, ok := event.(*models.SubmissionCompletedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := SubmissionCompletedDTO(*e)
		msg = observerv1.SubmissionCompletedMessage{
			EventType: new(observerv1.SubmissionCompletedMessageEventType),
			Payload:   &payload,
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
		Source:       &e.Source,
		State:        &e.State,
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
		MemoryStat:   &e.MemoryStat,
		FailedTest:   e.FailedTest,
		Penalty:      &e.Penalty,
		Position:     int32Ptr(e.Position),
		ProblemId:    e.ProblemId,
		ProblemTitle: strPtrIfNotEmpty(e.ProblemTitle),
		Score:        &e.Score,
		State:        &e.State,
		TimeStat:     &e.TimeStat,
		UserId:       e.UserId,
		Username:     strPtrIfNotEmpty(e.Username),
	}
}

func ContestEnvelopeDTO(eventType string, event any) ([]byte, error) {
	var msg observerv1.ContestMessage

	switch eventType {
	case models.ContestEventAnnouncementCreated:
		e, ok := event.(*models.ContestAnnouncementCreatedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := observerv1.MessageContestAnnouncementCreated{
			AuthorId:       &e.AuthorID,
			AuthorUsername: e.AuthorUsername,
			Body:           e.Body,
			ContestId:      e.ContestID,
			CreatedAt:      e.CreatedAt,
			Id:             e.ID,
			ProblemId:      e.ProblemID,
			ProblemLetter:  e.ProblemLetter,
			ProblemTitle:   e.ProblemTitle,
			Title:          e.Title,
		}
		msg = observerv1.ContestAnnouncementCreatedMessage{
			EventType: new(observerv1.ContestAnnouncementCreatedMessageEventType),
			Payload:   &payload,
		}
	case models.ContestEventAnnouncementDeleted:
		e, ok := event.(*models.ContestAnnouncementDeletedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := observerv1.MessageContestAnnouncementDeleted{
			ContestId: e.ContestID,
			Id:        e.ID,
		}
		msg = observerv1.ContestAnnouncementDeletedMessage{
			EventType: new(observerv1.ContestAnnouncementDeletedMessageEventType),
			Payload:   &payload,
		}
	case models.ContestEventClarificationCreated:
		e, ok := event.(*models.ContestClarificationCreatedEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		payload := observerv1.MessageContestClarificationCreated{
			ContestId:     e.ContestID,
			CreatedAt:     e.CreatedAt,
			Id:            e.ID,
			ProblemId:     e.ProblemID,
			ProblemLetter: e.ProblemLetter,
			ProblemTitle:  e.ProblemTitle,
			Question:      e.Question,
			UserId:        e.UserID,
			Username:      e.Username,
		}
		msg = observerv1.ContestClarificationCreatedMessage{
			EventType: new(observerv1.ContestClarificationCreatedMessageEventType),
			Payload:   &payload,
		}
	case models.ContestEventClarificationAnswered:
		e, ok := event.(*models.ContestClarificationAnsweredEvent)
		if !ok || e == nil {
			return nil, fmt.Errorf("invalid event payload for %s: %T", eventType, event)
		}
		var ansBy *uuid.UUID
		if e.AnsweredBy != uuid.Nil {
			ansBy = &e.AnsweredBy
		}
		var ansByUsername *string
		if e.AnsweredByUsername != "" {
			ansByUsername = &e.AnsweredByUsername
		}
		payload := observerv1.MessageContestClarificationAnswered{
			Answer:             e.Answer,
			AnsweredAt:         e.AnsweredAt,
			AnsweredBy:         ansBy,
			AnsweredByUsername: ansByUsername,
			ContestId:          e.ContestID,
			Id:                 e.ID,
			ProblemId:          e.ProblemID,
			ProblemLetter:      e.ProblemLetter,
			ProblemTitle:       e.ProblemTitle,
			Question:           e.Question,
			UserId:             e.UserID,
			Username:           e.Username,
		}
		msg = observerv1.ContestClarificationAnsweredMessage{
			EventType: new(observerv1.ContestClarificationAnsweredMessageEventType),
			Payload:   &payload,
		}
	default:
		return nil, fmt.Errorf("unknown contest event type: %s", eventType)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal contest message: %w", err)
	}

	return data, nil
}
