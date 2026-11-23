package observer

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const timeToClose = time.Second * 1

type Observer struct {
	upgrader    *websocket.Upgrader
	handler     http.Handler
	hub         *Hub
	contestsHub *ContestsHub
	auth        AuthUC
	access      access.Loader
	contests    ContestReader
}

func NewObserver(hub *Hub, contestsHub *ContestsHub, middleware func(http.Handler) http.Handler, auth AuthUC, loader access.Loader, contests ContestReader) *Observer {
	mux := http.NewServeMux()

	observer := &Observer{
		upgrader:    &websocket.Upgrader{},
		hub:         hub,
		contestsHub: contestsHub,
		auth:        auth,
		access:      loader,
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
	Topics        []string
	audience      string
	viewer        uuid.UUID
	configuration string
	ContestId     *uuid.UUID
	UserId        *uuid.UUID
	ProblemId     *uuid.UUID
	Language      *string
}

func metaMatches(meta *submissionsmodel.SubmissionEventMeta, filter *SubmissionsFilter) bool {
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
	case *submissionsmodel.SubmissionCreatedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *submissionsmodel.SubmissionQueuedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *submissionsmodel.SubmissionCompilingStartedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *submissionsmodel.SubmissionTestingStartedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *submissionsmodel.SubmissionTestStartedEvent:
		return metaMatches(&e.SubmissionEventMeta, f)
	case *submissionsmodel.SubmissionCompletedEvent:
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
	if values, ok := q["language"]; ok {
		if len(values) != 1 {
			return nil, fmt.Errorf("invalid language")
		}
		if values[0] == "" {
			return nil, fmt.Errorf("invalid language")
		}
		filter.Language = &values[0]
	}
	return filter, nil
}

func (s *Observer) HandleSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter, err := parseSubmissionsFilter(r)
	if err != nil {
		subscriptionError(w, r, pkg.Wrap(pkg.ErrInvalid, err, "invalid filter"))
		return
	}
	lease, err := s.admit(ctx, filter, false)
	if err != nil {
		subscriptionError(w, r, err)
		return
	}
	if err := s.hub.trackScoreboard(ctx, filter.ContestId); err != nil {
		subscriptionError(w, r, err)
		return
	}
	defer s.hub.releaseScoreboard(filter.ContestId)
	sub := s.hub.subscribe(filter)
	conn, err := s.upgrader.Upgrade(w, r, http.Header{"X-Request-ID": w.Header().Values("X-Request-ID")})
	if err != nil {
		sub.close("upgrade failed", err)
		slog.WarnContext(ctx, "upgrade websocket", "error", err)
		return
	}
	sub.attach(conn)
	sub.verify = lease
	sub.onInvalidate = func(err error) { s.sendFinalSnapshot(ctx, filter, sub.conn, err) }
	sub.run(func(event *WrappedEvent) error {
		return s.sendSubmission(ctx, filter, event, sub.conn)
	})
}

func (s *Observer) HandleContests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	parsed, err := parseSubmissionsFilter(r)
	if err != nil || parsed.ContestId == nil {
		subscriptionError(w, r, pkg.Wrap(pkg.ErrInvalid, err, "invalid filter"))
		return
	}
	lease, err := s.admit(ctx, parsed, true)
	if err != nil {
		subscriptionError(w, r, err)
		return
	}
	filter := &ContestsFilter{ContestId: *parsed.ContestId, Topics: parsed.Topics}

	sub := s.contestsHub.subscribe(filter)
	conn, err := s.upgrader.Upgrade(w, r, http.Header{"X-Request-ID": w.Header().Values("X-Request-ID")})
	if err != nil {
		sub.close("upgrade failed", err)
		slog.WarnContext(ctx, "upgrade contest websocket", "error", err)
		return
	}
	sub.attach(conn)
	sub.verify = lease
	sub.run(func(event *WrappedEvent) error {
		return s.sendContest(parsed, event, sub.conn)
	})
}
