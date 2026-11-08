package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type WrappedEvent struct {
	projectionContest       *contestsmodel.Contest
	topics                  map[string]topicProjection
	configuration           string
	Seq                     uint64
	EventType               string
	RawEvent                any
	DTOPayload              []byte
	scoreboardPublic        []byte
	scoreboardUnfrozen      []byte
	scoreboardConfiguration string
}

type Subscriber struct {
	*subscription
	filter *SubmissionsFilter
}

type Hub struct {
	revisions                RevisionReader
	contests                 ContestReader
	scoreboard               Scoreboard
	scoreboardMu             sync.Mutex
	scoreboardConfigurations map[uuid.UUID]string
	scoreboardReferences     map[uuid.UUID]int
	finalSnapshots           map[uuid.UUID]finalSnapshot
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

func NewHub(ringSize int, contests ContestReader, scoreboard Scoreboard, revisions ...RevisionReader) *Hub {
	h := &Hub{
		contests: contests, scoreboard: scoreboard, finalSnapshots: make(map[uuid.UUID]finalSnapshot), scoreboardConfigurations: make(map[uuid.UUID]string), scoreboardReferences: make(map[uuid.UUID]int),
		subscribers: make(map[uuid.UUID]*Subscriber),
		ring:        NewRingBuffer[*WrappedEvent](safeUint64(ringSize)),
	}
	if len(revisions) > 0 {
		h.revisions = revisions[0]
	}
	return h
}

func parseEvent[T any](payload []byte) (*T, error) {
	var e T
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, &eventdata.PermanentError{Cause: fmt.Errorf("failed to unmarshal event payload: %w", err)}
	}
	return &e, nil
}

func (h *Hub) Handle(ctx context.Context, eventType string, payload []byte) error {
	ctx = m.WithReadTime(ctx, time.Now().UTC())
	var revision int64
	if h.revisions != nil {
		var err error
		revision, err = h.revisions.AccessRevision(ctx)
		if err != nil {
			return err
		}
	}
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
	if err := h.projectTopics(ctx, wrapped); err != nil {
		return err
	}
	if h.revisions != nil {
		current, err := h.revisions.AccessRevision(ctx)
		if err != nil {
			return err
		}
		if current != revision {
			return pkg.ErrConflict
		}
	}
	h.broadcast(wrapped)

	return nil
}

// subscribe captures history and registers the live queue under the same lock as broadcast.
func (h *Hub) subscribe(conn websocketConnection, filter *SubmissionsFilter, replay bool) (*Subscriber, []*WrappedEvent, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	history, err := snapshotHistory(h.ring, filter.Since, replay)
	if err != nil {
		return nil, nil, err
	}
	sub := &Subscriber{subscription: newSubscription(conn), filter: filter}
	h.subscribers[sub.id] = sub
	return sub, scopeHistory(history, filter.Topics), nil
}

func (h *Hub) broadcast(event *WrappedEvent) {
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
		slog.Error("failed to push event to ring buffer", "error", err)
	}
}
