package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WrappedEvent struct {
	Seq                     uint64
	EventType               string
	RawEvent                any
	DTOPayload              []byte
	scoreboardPublic        []byte
	scoreboardUnfrozen      []byte
	scoreboardConfiguration string
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
	contests                 ContestsUC
	scoreboardMu             sync.Mutex
	scoreboardConfigurations map[uuid.UUID]string
	scoreboardReferences     map[uuid.UUID]int
	subscribers              map[uuid.UUID]*Subscriber
	ring                     *RingBuffer[*WrappedEvent]
	mu                       sync.RWMutex
}

func safeUint64(v int) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

func NewHub(ringSize int, contests ContestsUC) *Hub {
	return &Hub{
		contests: contests, scoreboardConfigurations: make(map[uuid.UUID]string), scoreboardReferences: make(map[uuid.UUID]int),
		subscribers: make(map[uuid.UUID]*Subscriber),
		ring:        NewRingBuffer[*WrappedEvent](safeUint64(ringSize)),
	}
}

func parseEvent[T any](payload []byte) (*T, error) {
	var e T
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, &eventdata.PermanentError{Cause: fmt.Errorf("failed to unmarshal event payload: %w", err)}
	}
	return &e, nil
}

func (h *Hub) Handle(ctx context.Context, eventType string, payload []byte) error {
	seq, ok := ctx.Value(eventdata.MessageStreamSequenceKey).(uint64)
	if !ok {
		slog.Error("missing message_stream_sequence")
		return nil
	}

	var event any
	var err error

	switch eventType {
	case submissionsmodel.SubmissionEventCreated:
		event, err = parseEvent[submissionsmodel.SubmissionCreatedEvent](payload)
	case submissionsmodel.SubmissionEventQueued:
		event, err = parseEvent[submissionsmodel.SubmissionQueuedEvent](payload)
	case submissionsmodel.SubmissionEventCompilingStarted:
		event, err = parseEvent[submissionsmodel.SubmissionCompilingStartedEvent](payload)
	case submissionsmodel.SubmissionEventTestingStarted:
		event, err = parseEvent[submissionsmodel.SubmissionTestingStartedEvent](payload)
	case submissionsmodel.SubmissionEventTestStarted:
		event, err = parseEvent[submissionsmodel.SubmissionTestStartedEvent](payload)
	case submissionsmodel.SubmissionEventCompleted:
		event, err = parseEvent[submissionsmodel.SubmissionCompletedEvent](payload)
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

	wrapped := &WrappedEvent{
		Seq:        seq,
		EventType:  eventType,
		RawEvent:   event,
		DTOPayload: clientPayload,
	}
	if err := h.projectScoreboard(ctx, wrapped); err != nil {
		return err
	}
	h.broadcast(wrapped)

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
