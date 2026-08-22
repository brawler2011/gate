package observer

import (
	"context"
	"log/slog"
	"sync"

	"github.com/brawler2011/gate/backend/internal/domain/models"
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
	Since       uint64
	ContestId   uuid.UUID
	UserId      *uuid.UUID
	IsModerator bool
}

func (f *ContestsFilter) Matches(event *WrappedEvent) bool {
	switch e := event.RawEvent.(type) {
	case *models.ContestAnnouncementCreatedEvent:
		return e.ContestID == f.ContestId
	case *models.ContestAnnouncementDeletedEvent:
		return e.ContestID == f.ContestId
	case *models.ContestClarificationCreatedEvent:
		if e.ContestID != f.ContestId {
			return false
		}
		if f.IsModerator {
			return true
		}
		return f.UserId != nil && *f.UserId == e.UserID
	case *models.ContestClarificationAnsweredEvent:
		if e.ContestID != f.ContestId {
			return false
		}
		if f.IsModerator {
			return true
		}
		return f.UserId != nil && *f.UserId == e.UserID
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
	seq, ok := ctx.Value(models.MessageStreamSequenceKey).(uint64)
	if !ok {
		slog.Error("missing message_stream_sequence in contest hub")
		return nil
	}

	var event any
	var err error

	switch eventType {
	case models.ContestEventAnnouncementCreated:
		event, err = parseEvent[models.ContestAnnouncementCreatedEvent](payload)
	case models.ContestEventAnnouncementDeleted:
		event, err = parseEvent[models.ContestAnnouncementDeletedEvent](payload)
	case models.ContestEventClarificationCreated:
		event, err = parseEvent[models.ContestClarificationCreatedEvent](payload)
	case models.ContestEventClarificationAnswered:
		event, err = parseEvent[models.ContestClarificationAnsweredEvent](payload)
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
