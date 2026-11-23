package observer

import (
	"context"
	"log/slog"
	"sync"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

type ContestSubscriber struct {
	*subscription
	filter *ContestsFilter
}

type ContestsFilter struct {
	Topics    []string
	ContestId uuid.UUID
}

func (f *ContestsFilter) Matches(event *WrappedEvent) bool {
	switch e := event.RawEvent.(type) {
	case *contestsmodel.ContestAnnouncementCreatedEvent:
		return e.ContestID == f.ContestId
	case *contestsmodel.ContestAnnouncementDeletedEvent:
		return e.ContestID == f.ContestId
	case *contestsmodel.ContestClarificationCreatedEvent:
		return e.ContestID == f.ContestId
	case *contestsmodel.ContestClarificationAnsweredEvent:
		return e.ContestID == f.ContestId
	default:
		slog.Warn("unknown event type in contest filter match", "type", event.EventType)
		return false
	}
}

type ContestsHub struct {
	subscribers map[uuid.UUID]*ContestSubscriber
	mu          sync.RWMutex
}

func NewContestsHub() *ContestsHub {
	return &ContestsHub{subscribers: make(map[uuid.UUID]*ContestSubscriber)}
}

func (h *ContestsHub) Handle(ctx context.Context, eventType string, payload []byte) error {
	var event any
	var err error

	switch eventType {
	case contestsmodel.ContestEventAnnouncementCreated:
		event, err = parseEvent[contestsmodel.ContestAnnouncementCreatedEvent](payload)
	case contestsmodel.ContestEventAnnouncementDeleted:
		event, err = parseEvent[contestsmodel.ContestAnnouncementDeletedEvent](payload)
	case contestsmodel.ContestEventClarificationCreated:
		event, err = parseEvent[contestsmodel.ContestClarificationCreatedEvent](payload)
	case contestsmodel.ContestEventClarificationAnswered:
		event, err = parseEvent[contestsmodel.ContestClarificationAnsweredEvent](payload)
	default:
		slog.WarnContext(ctx, "unknown contest event type in hub", "type", eventType)
		return nil
	}

	if err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal contest event", "type", eventType, "error", err)
		return nil
	}

	clientPayload, err := ContestEnvelopeDTO(eventType, event)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal contest client payload", "type", eventType, "error", err)
		return nil
	}
	if len(clientPayload) == 0 {
		slog.WarnContext(ctx, "empty contest client payload", "type", eventType)
		return nil
	}

	wrapped := &WrappedEvent{
		EventType:  eventType,
		RawEvent:   event,
		DTOPayload: clientPayload,
	}
	projectContestTopics(wrapped)
	h.broadcast(wrapped)

	return nil
}

func (h *ContestsHub) subscribe(filter *ContestsFilter) *ContestSubscriber {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub := &ContestSubscriber{subscription: newSubscription(), filter: filter}
	h.subscribers[sub.id] = sub
	sub.detach = func() {
		h.mu.Lock()
		delete(h.subscribers, sub.id)
		h.mu.Unlock()
	}
	return sub
}

func (h *ContestsHub) closeAll(reason string) {
	h.mu.Lock()
	subs := make([]*subscription, 0, len(h.subscribers))
	for _, sub := range h.subscribers {
		subs = append(subs, sub.subscription)
	}
	h.mu.Unlock()
	for _, sub := range subs {
		sub.close(reason, nil)
	}
}

func (h *ContestsHub) broadcast(event *WrappedEvent) {
	var overflow []*subscription
	h.mu.Lock()
	for id, sub := range h.subscribers {
		if sub.filter.Matches(event) {
			if !sub.enqueue(scopeEvent(event, sub.filter.Topics)) {
				delete(h.subscribers, id)
				overflow = append(overflow, sub.subscription)
			}
		}
	}
	h.mu.Unlock()

	for _, sub := range overflow {
		sub.close("event queue overflow", nil)
	}
}
