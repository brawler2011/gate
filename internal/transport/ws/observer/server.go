package observer

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Observer struct {
	upgrader      *websocket.Upgrader
	server        *http.Server
	hub           *Hub
	contestsUC    interfaces.ContestsUC
	permissionsUC interfaces.PermissionsUC
}

func NewObserver(addr string, ringSize int) *Observer {
	mux := http.NewServeMux()
	hub := NewHub(ringSize)

	observer := &Observer{
		upgrader: &websocket.Upgrader{},
		hub:      hub,
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}

	mux.HandleFunc("/ws/submissions", observer.HandleSubmissions)

	return observer
}

func (s *Observer) Start() {
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("failed to start http server", "error", err)
	}
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
	switch e := event.Event.(type) {
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

		contestPermissions, err := s.permissionsUC.GetContestPermissions(ctx, *filter.ContestId, user.Id)
		// FIXME: this error can actually mean anything in this world
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if !contestPermissions.GetContest {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// shit
		if !contestPermissions.ListUsersSubmissions && (filter.UserId == nil || filter.UserId != nil && *filter.UserId != user.Id) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// shit
		if !contestPermissions.ListOwnSubmissions && filter.UserId != nil && *filter.UserId == user.Id {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// what about the fucking SUBMISSION page ???
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

	// 3. Catch-up from ring buffer
	if filter.Since < s.hub.ring.MinSeq() && filter.Since != 0 {
		slog.Warn("client since too old", "since", filter.Since, "min", s.hub.ring.MinSeq())
		// In a real app, you might send an error message here before closing
		return
	}

	history, err := s.hub.ring.GetRange(filter.Since, sub.barrierSeq)
	if err != nil {
		slog.Error("failed to get history", "error", err)
		return
	}
	for _, event := range history {
		if filter.Matches(event) {
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		}
	}

	// 4. Flush pending and transition to LIVE
	sub.mu.Lock()
	for _, event := range sub.pending {
		if err := conn.WriteJSON(event); err != nil {
			sub.mu.Unlock()
			return
		}
	}
	sub.pending = nil
	sub.state = StateLive
	sub.mu.Unlock()

	// 5. Main write loop (live events)
	for event := range sub.outbox {
		if err := conn.WriteJSON(event); err != nil {
			break
		}
	}
}
