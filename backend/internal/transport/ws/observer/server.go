package observer

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	observerv1 "github.com/gate149/contracts/observer/v1"
	"github.com/gate149/gate/backend/config"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const timeToClose = time.Second * 1

type Observer struct {
	upgrader *websocket.Upgrader
	server   *http.Server
	hub      *Hub
}

func NewObserver(cfg *config.WsConfig, hub *Hub, middleware func(http.Handler) http.Handler) *Observer {
	mux := http.NewServeMux()

	allowedOrigins := make(map[string]bool, 2)
	for _, origin := range strings.Split(strings.TrimSpace(cfg.AllowedOrigins), ",") {
		allowedOrigins[origin] = true
	}

	observer := &Observer{
		upgrader: &websocket.Upgrader{
			CheckOrigin: newCheckOrigin(allowedOrigins, cfg.Env),
		},
		hub: hub,
		server: &http.Server{
			Addr:    cfg.WsAddress,
			Handler: middleware(mux),
		},
	}

	mux.HandleFunc("/ws/submissions", observer.HandleSubmissions)

	return observer
}

func newCheckOrigin(allowedOrigins map[string]bool, env string) func(r *http.Request) bool {
	if env == "local" {
		slog.Info("local environment, all origins allowed")
		return func(r *http.Request) bool {
			return true
		}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			return true
		}
		return false
	}
}

func (s *Observer) Start() error {
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Observer) Close(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

type SubmissionsFilter struct {
	Since     uint64
	ContestId *uuid.UUID
	UserId    *uuid.UUID
	ProblemId *uuid.UUID
	Language  *int32
}

func metaMatches(meta *models.SubmissionEventMeta, filter *SubmissionsFilter) bool {
	if filter.ContestId != nil && meta.ContestId != nil && *meta.ContestId != *filter.ContestId {
		return false
	}
	if filter.UserId != nil && meta.UserId != nil && *meta.UserId != *filter.UserId {
		return false
	}
	if filter.ProblemId != nil && meta.ProblemId != nil && *meta.ProblemId != *filter.ProblemId {
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
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		since, err := strconv.ParseUint(sinceStr, 10, 64)
		if err == nil {
			filter.Since = since
		}
	}

	if cId := r.URL.Query().Get("contestId"); cId != "" {
		if id, err := uuid.Parse(cId); err == nil {
			filter.ContestId = &id
		}
	}

	if uId := r.URL.Query().Get("userId"); uId != "" {
		if id, err := uuid.Parse(uId); err == nil {
			filter.UserId = &id
		}
	}

	if pId := r.URL.Query().Get("problemId"); pId != "" {
		if id, err := uuid.Parse(pId); err == nil {
			filter.ProblemId = &id
		}
	}

	if langStr := r.URL.Query().Get("language"); langStr != "" {
		if lang, err := strconv.ParseInt(langStr, 10, 32); err == nil {
			l32 := int32(lang)
			filter.Language = &l32
		}
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

	filter, _ := parseSubmissionsFilter(r)

	if !user.IsAdmin() {
		// Non-admins can subscribe only to a specific contest
		if filter.ContestId == nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// TODO: verify the user is a participant or organizer of the requested contest.
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade connection", "error", err)
		return
	}

	sub := &Subscriber{
		id:      uuid.New(),
		conn:    conn,
		filter:  filter,
		outbox:  make(chan *WrappedEvent, 1000),
		state:   StatePaused,
		pending: make([]*WrappedEvent, 0),
	}

	// 1. Set barrier
	sub.barrierSeq = s.hub.ring.MaxSeq()

	// 2. Register as PAUSED in Hub
	s.hub.mu.Lock()
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
	history, err := s.hub.ring.GetRange(filter.Since, sub.barrierSeq)
	if err != nil && !errors.Is(err, ErrBufferEmpty) {
		code, message := getClosingFrameData(err)
		if err := conn.WriteControl(code, message, time.Now().Add(timeToClose)); err != nil {
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
			if err := conn.WriteMessage(websocket.TextMessage, event.DTOPayload); err != nil {
				handleWriteError(err)
				return
			}
		}
	}

	// 4. Flush pending and transition to LIVE
	sub.mu.Lock()
	for _, event := range sub.pending {
		select {
		case <-readDone:
			sub.mu.Unlock()
			return
		default:
		}

		if err := conn.WriteMessage(websocket.TextMessage, event.DTOPayload); err != nil {
			handleWriteError(err)
			sub.mu.Unlock()
			return
		}
	}

	sub.pending = nil
	sub.state = StateLive
	sub.mu.Unlock()

	// 5. Main write loop (live events)
	for {
		select {
		case <-readDone:
			return
		case event := <-sub.outbox:
			if event == nil {
				slog.Error("event is nil")
				if err := conn.WriteControl(websocket.CloseInternalServerErr, nil, time.Now().Add(timeToClose)); err != nil {
					handleWriteError(err)
				}
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, event.DTOPayload); err != nil {
				handleWriteError(err)
				return
			}
		}
	}
}

func getClosingFrameData(err error) (int, []byte) {
	switch {
	case errors.Is(err, ErrBufferEmpty):
		return int(observerv1.SubmissionsWsCloseCodeBufferEmpty),
			[]byte(observerv1.SubmissionsWsCloseReasonBufferEmpty)
	case errors.Is(err, ErrInvalidRange):
		return int(observerv1.SubmissionsWsCloseCodeInvalidRange),
			[]byte(observerv1.SubmissionsWsCloseReasonInvalidRange)
	case errors.Is(err, ErrHistoryLost):
		return int(observerv1.SubmissionsWsCloseCodeHistoryLost),
			[]byte(observerv1.SubmissionsWsCloseReasonHistoryLost)
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
