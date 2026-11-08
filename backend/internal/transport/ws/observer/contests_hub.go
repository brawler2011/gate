package observer

import (
	"context"
	"log/slog"
	"sync"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
)

type ContestSubscriber struct {
	*subscription
	filter *ContestsFilter
}

type ContestsFilter struct {
	Topics    []string
	Since     uint64
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
	ring        *RingBuffer[*WrappedEvent]
	mu          sync.RWMutex
}

func NewContestsHub(ringSize int) *ContestsHub {
	return &ContestsHub{
		subscribers: make(map[uuid.UUID]*ContestSubscriber),
		ring:        NewRingBuffer[*WrappedEvent](safeUint64(ringSize)),
	}
}

func (h *ContestsHub) Handle(ctx context.Context, eventType string, payload []byte) error {
	seq, ok := ctx.Value(eventdata.MessageStreamSequenceKey).(uint64)
	if !ok {
		slog.Error("missing message_stream_sequence in contest hub")
		return nil
	}

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
		slog.Warn("unknown contest event type in hub", "type", eventType)
		return nil
	}

	if err != nil {
		slog.Error("failed to unmarshal contest event", "type", eventType, "error", err)
		return nil
	}

	clientPayload, err := ContestEnvelopeDTO(eventType, event)
	if err != nil {
		slog.Error("failed to marshal contest client payload", "type", eventType, "error", err)
		return nil
	}
	if len(clientPayload) == 0 {
		slog.Warn("empty contest client payload", "type", eventType)
		return nil
	}

	wrapped := &WrappedEvent{
		Seq:        seq,
		EventType:  eventType,
		RawEvent:   event,
		DTOPayload: clientPayload,
	}
	projectContestTopics(wrapped)
	h.broadcast(wrapped)

	return nil
}

// subscribe captures history and registers the live queue under the same lock as broadcast.
func (h *ContestsHub) subscribe(conn websocketConnection, filter *ContestsFilter, replay bool) (*ContestSubscriber, []*WrappedEvent, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	history, err := snapshotHistory(h.ring, filter.Since, replay)
	if err != nil {
		return nil, nil, err
	}
	sub := &ContestSubscriber{subscription: newSubscription(conn), filter: filter}
	h.subscribers[sub.id] = sub
	return sub, scopeHistory(history, filter.Topics), nil
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
	err := h.ring.Push(event, event.Seq)
	h.mu.Unlock()

	for _, sub := range overflow {
		sub.close("event queue overflow", nil)
	}
	if err != nil {
		slog.Error("failed to push contest event to ring buffer", "error", err)
	}
}
