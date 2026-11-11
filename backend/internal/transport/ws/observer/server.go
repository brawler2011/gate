package observer

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	observerv1 "github.com/brawler2011/gate/backend/contracts/observer/v1"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
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
	permissions PermissionsUC
	contests    ContestReader
	revisions   RevisionReader
}

func NewObserver(hub *Hub, contestsHub *ContestsHub, middleware func(http.Handler) http.Handler, auth AuthUC, permissions PermissionsUC, contests ContestReader, revisions ...RevisionReader) *Observer {
	mux := http.NewServeMux()

	observer := &Observer{
		upgrader:    &websocket.Upgrader{},
		hub:         hub,
		contestsHub: contestsHub,
		auth:        auth,
		permissions: permissions,
		contests:    contests,
	}

	if len(revisions) > 0 {
		observer.revisions = revisions[0]
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
	Since         uint64
	ContestId     *uuid.UUID
	UserId        *uuid.UUID
	ProblemId     *uuid.UUID
	Language      *int32
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
		if err := submissionsmodel.LanguageNameValid(lang); err != nil {
			return nil, err
		}
		filter.Language = &lang
	}
	return filter, nil
}

func (s *Observer) HandleSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter, err := parseSubmissionsFilter(r)
	if err != nil {
		http.Error(w, "invalid filter", http.StatusBadRequest)
		return
	}
	lease, err := s.admit(ctx, filter, false)
	if err != nil {
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

	sub, history, err := s.hub.subscribe(conn, filter, r.URL.Query().Has("since"))
	if err != nil {
		rejectHistory(conn, err)
		return
	}
	sub.verify = lease
	sub.onInvalidate = func(err error) { s.sendFinalSnapshot(ctx, filter, sub.conn, err) }
	sub.run(history, filter.Matches, func(event *WrappedEvent) error {
		return s.sendSubmission(ctx, filter, event, sub.conn)
	})
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

func (s *Observer) HandleContests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	parsed, err := parseSubmissionsFilter(r)
	if err != nil || parsed.ContestId == nil {
		http.Error(w, "invalid filter", http.StatusBadRequest)
		return
	}
	lease, err := s.admit(ctx, parsed, true)
	if err != nil {
		subscriptionError(w, err)
		return
	}
	filter := &ContestsFilter{Since: parsed.Since, ContestId: *parsed.ContestId, Topics: parsed.Topics}

	conn, err := s.upgrader.Upgrade(w, r, http.Header{"X-Request-ID": w.Header().Values("X-Request-ID")})
	if err != nil {
		slog.Error("failed to upgrade contest connection", "error", err)
		return
	}

	sub, history, err := s.contestsHub.subscribe(conn, filter, r.URL.Query().Has("since"))
	if err != nil {
		rejectHistory(conn, err)
		return
	}
	sub.verify = lease
	sub.run(history, filter.Matches, func(event *WrappedEvent) error {
		return s.sendContest(parsed, event, sub.conn)
	})
}
