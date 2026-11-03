package observer

import (
	"context"
	"log/slog"
	"sync"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type ContestSubscriber struct {
	id         uuid.UUID
	conn       *websocket.Conn
	filter     *ContestsFilter
	outbox     chan *WrappedEvent
	pending    []*WrappedEvent
	state      SubscriberState
	barrierSeq uint64
	mu         sync.Mutex
}

type ContestsFilter struct {
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

	h.broadcast(&WrappedEvent{
		Seq:        seq,
		EventType:  eventType,
		RawEvent:   event,
		DTOPayload: clientPayload,
	})

	return nil
}

func (h *ContestsHub) broadcast(event *WrappedEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscribers {
		sub.mu.Lock()
		if sub.state == StatePaused {
			if event.Seq > sub.barrierSeq && sub.filter.Matches(event) {
				sub.pending = append(sub.pending, event)
			}
		} else {
			if sub.filter.Matches(event) {
				select {
				case sub.outbox <- event:
				default:
					slog.Warn("contest subscriber outbox full, closing", "id", sub.id)
					sub.conn.Close()
				}
			}
		}
		sub.mu.Unlock()
	}

	err := h.ring.Push(event, event.Seq)
	if err != nil {
		slog.Error("failed to push contest event to ring buffer", "error", err)
	}
}
