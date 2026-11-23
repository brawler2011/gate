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

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type WrappedEvent struct {
	projectionContest       *contestsmodel.Contest
	topics                  map[string]topicProjection
	configuration           string
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
	contests                 ContestReader
	scoreboard               Scoreboard
	scoreboardMu             sync.Mutex
	scoreboardConfigurations map[uuid.UUID]string
	scoreboardReferences     map[uuid.UUID]int
	finalSnapshots           map[uuid.UUID]finalSnapshot
	subscribers              map[uuid.UUID]*Subscriber
	mu                       sync.RWMutex
}

func NewHub(contests ContestReader, scoreboard Scoreboard) *Hub {
	return &Hub{
		contests: contests, scoreboard: scoreboard, finalSnapshots: make(map[uuid.UUID]finalSnapshot), scoreboardConfigurations: make(map[uuid.UUID]string), scoreboardReferences: make(map[uuid.UUID]int),
		subscribers: make(map[uuid.UUID]*Subscriber),
	}
}

func parseEvent[T any](payload []byte) (*T, error) {
	var e T
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event payload: %w", err)
	}
	return &e, nil
}

func (h *Hub) Handle(ctx context.Context, eventType string, payload []byte) error {
	ctx = m.WithReadTime(ctx, time.Now().UTC())
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
		slog.WarnContext(ctx, "unknown event type", "type", eventType)
		return nil
	}

	if err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal event", "type", eventType, "error", err)
		return nil
	}

	clientPayload, err := SubmissionsEnvelopeDTO(eventType, event)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal client payload", "type", eventType, "error", err)
		return nil
	}
	if len(clientPayload) == 0 {
		slog.WarnContext(ctx, "empty client payload", "type", eventType)
		return nil
	}

	wrapped := &WrappedEvent{
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
	h.broadcast(wrapped)

	return nil
}

func (h *Hub) subscribe(filter *SubmissionsFilter) *Subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub := &Subscriber{subscription: newSubscription(), filter: filter}
	h.subscribers[sub.id] = sub
	sub.detach = func() {
		h.mu.Lock()
		delete(h.subscribers, sub.id)
		h.mu.Unlock()
	}
	return sub
}

// closeAll makes every client reconnect and refetch its snapshot.
func (h *Hub) closeAll(reason string) {
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
	h.mu.Unlock()

	for _, sub := range overflow {
		sub.close("event queue overflow", nil)
	}
}
