package observer

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WrappedEvent struct {
	Seq        uint64
	EventType  string
	RawEvent   any
	DTOPayload []byte
}

type SubscriberState int

const (
	StatePaused SubscriberState = iota
	StateLive
)

type Subscriber struct {
	id         uuid.UUID
	conn       *websocket.Conn
	filter     *SubmissionsFilter
	outbox     chan *WrappedEvent
	pending    []*WrappedEvent
	state      SubscriberState
	barrierSeq uint64
	mu         sync.Mutex
}

type Hub struct {
	subscribers map[uuid.UUID]*Subscriber
	ring        *RingBuffer[*WrappedEvent]
	mu          sync.RWMutex
}

func NewHub(ringSize int) *Hub {
	return &Hub{
		subscribers: make(map[uuid.UUID]*Subscriber),
		ring:        NewRingBuffer[*WrappedEvent](uint64(ringSize)),
	}
}

func parseEvent[T any](payload []byte) (*T, error) {
	var e T
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (h *Hub) Handle(ctx context.Context, eventType string, payload []byte) error {
	seq, ok := ctx.Value("message_stream_sequence").(uint64)
	if !ok {
		slog.Error("missing message_stream_sequence")
		return nil
	}

	var event any
	var err error

	switch eventType {
	case models.SubmissionEventCreated:
		event, err = parseEvent[models.SubmissionCreatedEvent](payload)
	case models.SubmissionEventQueued:
		event, err = parseEvent[models.SubmissionQueuedEvent](payload)
	case models.SubmissionEventCompilingStarted:
		event, err = parseEvent[models.SubmissionCompilingStartedEvent](payload)
	case models.SubmissionEventTestingStarted:
		event, err = parseEvent[models.SubmissionTestingStartedEvent](payload)
	case models.SubmissionEventTestStarted:
		event, err = parseEvent[models.SubmissionTestStartedEvent](payload)
	case models.SubmissionEventCompleted:
		event, err = parseEvent[models.SubmissionCompletedEvent](payload)
	default:
		slog.Warn("unknown event type", "type", eventType)
		return nil
	}

	if err != nil {
		slog.Error("failed to unmarshal event", "type", eventType, "error", err)
		return nil
	}

	clientPayload, err := SubmissionsEnvelopeDTO(eventType, event)
	if err != nil {
		slog.Error("failed to marshal client payload", "type", eventType, "error", err)
		return nil
	}
	if len(clientPayload) == 0 {
		slog.Warn("empty client payload", "type", eventType)
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
func (h *Hub) broadcast(event *WrappedEvent) {
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
					// Slow client policy: close connection
					slog.Warn("subscriber outbox full, closing", "id", sub.id)
					sub.conn.Close()
				}
			}
		}
		sub.mu.Unlock()
	}

	err := h.ring.Push(event, event.Seq)
	if err != nil {
		slog.Error("failed to push event to ring buffer", "error", err)
	}
}
