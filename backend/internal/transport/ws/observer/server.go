package observer

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	observerv1 "github.com/brawler2011/contracts/observer/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const timeToClose = time.Second * 1

type Observer struct {
	upgrader    *websocket.Upgrader
	handler     http.Handler
	hub         *Hub
	contestsHub *ContestsHub
	auth        interfaces.AuthUC
	permissions interfaces.PermissionsUC
	contests    interfaces.ContestsUC
}

func NewObserver(hub *Hub, contestsHub *ContestsHub, middleware func(http.Handler) http.Handler, auth interfaces.AuthUC, permissions interfaces.PermissionsUC, contests interfaces.ContestsUC) *Observer {
	mux := http.NewServeMux()

	observer := &Observer{
		upgrader:    &websocket.Upgrader{},
		hub:         hub,
		contestsHub: contestsHub,
		auth:        auth,
		permissions: permissions,
		contests:    contests,
	}

	mux.HandleFunc("/ws/submissions", observer.HandleSubmissions)
	mux.HandleFunc("/ws/contests", observer.HandleContests)
	observer.handler = middleware(mux)

	return observer
}

func (s *Observer) Handler() http.Handler {
	return s.handler
}

type SubmissionsFilter struct {
	Since     uint64
	ContestId *uuid.UUID
	UserId    *uuid.UUID
	ProblemId *uuid.UUID
	Language  *int32
}

func metaMatches(meta *models.SubmissionEventMeta, filter *SubmissionsFilter) bool {
	if filter.ContestId != nil && (meta.ContestId == nil || *meta.ContestId != *filter.ContestId) {
		return false
	}
	if filter.UserId != nil && (meta.UserId == nil || *meta.UserId != *filter.UserId) {
		return false
	}
	if filter.ProblemId != nil && (meta.ProblemId == nil || *meta.ProblemId != *filter.ProblemId) {
		return false
	}
	if filter.Language != nil && meta.Language != *filter.Language {
		return false
	}
	return true
}

func (f *SubmissionsFilter) Matches(event *WrappedEvent) bool {
	switch e := event.RawEvent.(type) {
	case *models.SubmissionCreatedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *models.SubmissionQueuedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *models.SubmissionCompilingStartedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *models.SubmissionTestingStartedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *models.SubmissionTestStartedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *models.SubmissionCompletedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	default:
		slog.Warn("unknown event type in filter match", "type", event.EventType)
		return false
	}
}

func parseSubmissionsFilter(r *http.Request) (*SubmissionsFilter, error) {
	filter := &SubmissionsFilter{}
	q := r.URL.Query()
	for key, target := range map[string]**uuid.UUID{"contestId": &filter.ContestId, "userId": &filter.UserId, "problemId": &filter.ProblemId} {
		if values, ok := q[key]; ok {
			if len(values) != 1 {
				return nil, fmt.Errorf("invalid %s", key)
			}
			id, err := uuid.Parse(values[0])
			if err != nil || id == uuid.Nil {
				return nil, fmt.Errorf("invalid %s", key)
			}
			*target = &id
		}
	}
	if values, ok := q["since"]; ok {
		if len(values) != 1 {
			return nil, fmt.Errorf("invalid since")
		}
		value, err := strconv.ParseUint(values[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid since: %w", err)
		}
		filter.Since = value
	}
	if values, ok := q["language"]; ok {
		if len(values) != 1 {
			return nil, fmt.Errorf("invalid language")
		}
		value, err := strconv.ParseInt(values[0], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid language: %w", err)
		}
		lang := int32(value)
		if err := models.LanguageNameValid(lang); err != nil {
			return nil, err
		}
		filter.Language = &lang
	}
	return filter, nil
}

func (s *Observer) HandleSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	filter, err := parseSubmissionsFilter(r)
	if err != nil {
		http.Error(w, "invalid filter", http.StatusBadRequest)
		return
	}
	if err := s.checkSubscription(ctx, filter, false); err != nil {
		subscriptionError(w, err)
		return
	}
	if err := s.hub.trackScoreboard(ctx, filter.ContestId); err != nil {
		subscriptionError(w, err)
		return
	}
	defer s.hub.releaseScoreboard(filter.ContestId)
	conn, err := s.upgrader.Upgrade(w, r, http.Header{"X-Request-ID": w.Header().Values("X-Request-ID")})
	if err != nil {
		slog.Error("failed to upgrade connection", "error", err)
		return
	}

	send := func(event *WrappedEvent) error { return s.sendSubmission(ctx, filter, event, conn) }

	sub := &Subscriber{
		id:      uuid.New(),
		conn:    conn,
		filter:  filter,
		outbox:  make(chan *WrappedEvent, 1000),
		state:   StatePaused,
		pending: make([]*WrappedEvent, 0),
	}

	// Set the barrier and register as PAUSED atomically with broadcast.
	s.hub.mu.Lock()
	sub.barrierSeq = s.hub.ring.MaxSeq()
	if !r.URL.Query().Has("since") {
		filter.Since = sub.barrierSeq
	}
	s.hub.subscribers[sub.id] = sub
	s.hub.mu.Unlock()

	// Cleanup on disconnect
	defer func() {
		s.hub.mu.Lock()
		delete(s.hub.subscribers, sub.id)
		s.hub.mu.Unlock()
		conn.Close()
	}()

	readDone := startReadPump(conn)

	// 3. Catch-up from ring buffer
	var history []*WrappedEvent
	if sub.barrierSeq != 0 {
		history, err = s.hub.ring.GetRange(filter.Since, sub.barrierSeq)
	}
	if err != nil && !errors.Is(err, ErrBufferEmpty) {
		code, message := getClosingFrameData(err)
		if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, string(message)), time.Now().Add(timeToClose)); err != nil {
			handleWriteError(err)
			return
		}
		slog.Error("cannot get history", "error", err, "since", filter.Since, "barrier", sub.barrierSeq)
		return
	}

	for _, event := range history {
		select {
		case <-readDone:
			return
		default:
		}

		if filter.Matches(event) {
			if err := send(event); err != nil {
				handleWriteError(err)
				return
			}
		}
	}

	// Drain batches outside locks; switch to live atomically after the last batch.
	for {
		sub.mu.Lock()
		batch := sub.pending
		sub.pending = nil
		if len(batch) == 0 {
			sub.state = StateLive
			sub.mu.Unlock()
			break
		}
		sub.mu.Unlock()
		for _, event := range batch {
			select {
			case <-readDone:
				return
			default:
			}
			if err := send(event); err != nil {
				handleWriteError(err)
				return
			}
		}
	}

	// 5. Main write loop (live events)
	for {
		select {
		case <-readDone:
			return
		case event := <-sub.outbox:
			if event == nil {
				slog.Error("event is nil")
				if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "internal error"), time.Now().Add(timeToClose)); err != nil {
					handleWriteError(err)
				}
				return
			}
			if err := send(event); err != nil {
				handleWriteError(err)
				return
			}
		}
	}
}

func getClosingFrameData(err error) (int, []byte) {
	switch {
	case errors.Is(err, ErrBufferEmpty):
		return observerv1.SubmissionsWsCloseCodeNumber_4408.Value().(int),
			[]byte(observerv1.SubmissionsWsCloseReasonBufferEmpty.Value().(string))
	case errors.Is(err, ErrInvalidRange):
		return observerv1.SubmissionsWsCloseCodeNumber_4410.Value().(int),
			[]byte(observerv1.SubmissionsWsCloseReasonInvalidRange.Value().(string))
	case errors.Is(err, ErrHistoryLost):
		return observerv1.SubmissionsWsCloseCodeNumber_4409.Value().(int),
			[]byte(observerv1.SubmissionsWsCloseReasonHistoryLost.Value().(string))
	default:
		return websocket.CloseInternalServerErr,
			[]byte(err.Error())
	}
}

func handleReadError(err error) {
	if websocket.IsCloseError(err,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseNormalClosure) {
		return
	}

	slog.Error("cannot read client message", "error", err)
}

func handleWriteError(err error) {
	if websocket.IsCloseError(err,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseNormalClosure) {
		return
	}

	slog.Error("cannot write message to client", "error", err)
}

func startReadPump(conn *websocket.Conn) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				handleReadError(err)
				return
			}
		}
	}()
	return done
}

func (s *Observer) HandleContests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	parsed, err := parseSubmissionsFilter(r)
	if err != nil || parsed.ContestId == nil {
		http.Error(w, "invalid filter", http.StatusBadRequest)
		return
	}
	if err := s.checkSubscription(ctx, parsed, true); err != nil {
		subscriptionError(w, err)
		return
	}
	filter := &ContestsFilter{Since: parsed.Since, ContestId: *parsed.ContestId}

	conn, err := s.upgrader.Upgrade(w, r, http.Header{"X-Request-ID": w.Header().Values("X-Request-ID")})
	if err != nil {
		slog.Error("failed to upgrade contest connection", "error", err)
		return
	}

	send := func(event *WrappedEvent) error { return s.sendContest(ctx, parsed, event, conn) }

	sub := &ContestSubscriber{
		id:      uuid.New(),
		conn:    conn,
		filter:  filter,
		outbox:  make(chan *WrappedEvent, 1000),
		state:   StatePaused,
		pending: make([]*WrappedEvent, 0),
	}

	s.contestsHub.mu.Lock()
	sub.barrierSeq = s.contestsHub.ring.MaxSeq()
	s.contestsHub.subscribers[sub.id] = sub
	s.contestsHub.mu.Unlock()

	defer func() {
		s.contestsHub.mu.Lock()
		delete(s.contestsHub.subscribers, sub.id)
		s.contestsHub.mu.Unlock()
		conn.Close()
	}()

	readDone := startReadPump(conn)

	// Catch-up from ring buffer
	var history []*WrappedEvent
	if sub.barrierSeq != 0 {
		history, err = s.contestsHub.ring.GetRange(filter.Since, sub.barrierSeq)
	}
	if err != nil && !errors.Is(err, ErrBufferEmpty) {
		code, message := getClosingFrameData(err)
		if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, string(message)), time.Now().Add(timeToClose)); err != nil {
			handleWriteError(err)
			return
		}
		slog.Error("cannot get contest history", "error", err, "since", filter.Since, "barrier", sub.barrierSeq)
		return
	}

	for _, event := range history {
		select {
		case <-readDone:
			return
		default:
		}

		if filter.Matches(event) {
			if err := send(event); err != nil {
				handleWriteError(err)
				return
			}
		}
	}

	// Drain batches outside locks; switch to live atomically after the last batch.
	for {
		sub.mu.Lock()
		batch := sub.pending
		sub.pending = nil
		if len(batch) == 0 {
			sub.state = StateLive
			sub.mu.Unlock()
			break
		}
		sub.mu.Unlock()
		for _, event := range batch {
			select {
			case <-readDone:
				return
			default:
			}
			if err := send(event); err != nil {
				handleWriteError(err)
				return
			}
		}
	}

	// Main write loop
	for {
		select {
		case <-readDone:
			return
		case event := <-sub.outbox:
			if event == nil {
				slog.Error("contest event is nil")
				if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "internal error"), time.Now().Add(timeToClose)); err != nil {
					handleWriteError(err)
				}
				return
			}
			if err := send(event); err != nil {
				handleWriteError(err)
				return
			}
		}
	}
}
