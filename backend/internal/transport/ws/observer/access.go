package observer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/gorilla/websocket"
)

func subscriptionError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	if errors.Is(err, pkg.NoPermission) || errors.Is(err, pkg.ErrNotFound) {
		code = http.StatusForbidden
	}
	if errors.Is(err, pkg.ErrUnauthenticated) {
		code = http.StatusUnauthorized
	}
	http.Error(w, http.StatusText(code), code)
}

func (s *Observer) currentAccess(ctx context.Context, filter *SubmissionsFilter) (models.User, *models.ContestPermissions, error) {
	session, err := middleware.GetSession(ctx)
	if err != nil {
		return models.User{}, nil, err
	}
	user, err := s.auth.Authenticate(ctx, session.ID)
	if err != nil {
		return models.User{}, nil, err
	}
	if user.IsGuest() || user.Id != session.UserID {
		return user, nil, pkg.ErrUnauthenticated
	}
	if filter.ContestId == nil {
		if !user.IsAdmin() {
			return user, nil, pkg.NoPermission
		}
		return user, nil, nil
	}
	permissions, err := s.permissions.GetContestPermissions(ctx, *filter.ContestId, user.Id)
	if err != nil {
		return user, nil, err
	}
	return user, permissions, nil
}

func submissionAccess(user models.User, permissions *models.ContestPermissions, filter *SubmissionsFilter) bool {
	if filter.ContestId == nil {
		return user.IsAdmin()
	}
	return permissions.ListUsersSubmissions || permissions.GetMonitor || (permissions.ListOwnSubmissions && (filter.UserId == nil || *filter.UserId == user.Id))
}

func (s *Observer) checkSubscription(ctx context.Context, filter *SubmissionsFilter, contest bool) error {
	user, permissions, err := s.currentAccess(ctx, filter)
	if err != nil {
		return err
	}
	if contest {
		if !permissions.GetContest {
			return pkg.NoPermission
		}
	} else if !submissionAccess(user, permissions, filter) {
		return pkg.NoPermission
	}
	return nil
}

func closeVerification(conn *websocket.Conn, err error) error {
	code := websocket.CloseInternalServerErr
	if errors.Is(err, pkg.NoPermission) || errors.Is(err, pkg.ErrNotFound) || errors.Is(err, pkg.ErrUnauthenticated) {
		code = websocket.ClosePolicyViolation
	}
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, http.StatusText(pkg.ToREST(err))), time.Now().Add(timeToClose))
	return err
}

func eventMeta(event *WrappedEvent) *models.SubmissionEventMeta {
	switch e := event.RawEvent.(type) {
	case *models.SubmissionCreatedEvent:
		return &e.SubmissionEventMeta
	case *models.SubmissionQueuedEvent:
		return &e.SubmissionEventMeta
	case *models.SubmissionCompilingStartedEvent:
		return &e.SubmissionEventMeta
	case *models.SubmissionTestingStartedEvent:
		return &e.SubmissionEventMeta
	case *models.SubmissionTestStartedEvent:
		return &e.SubmissionEventMeta
	case *models.SubmissionCompletedEvent:
		return &e.SubmissionEventMeta
	default:
		return nil
	}
}

func (s *Observer) sendSubmission(ctx context.Context, filter *SubmissionsFilter, event *WrappedEvent, conn *websocket.Conn) error {
	user, permissions, err := s.currentAccess(ctx, filter)
	if err != nil {
		return closeVerification(conn, err)
	}
	if !submissionAccess(user, permissions, filter) {
		return closeVerification(conn, pkg.NoPermission)
	}
	if !filter.Matches(event) {
		return nil
	}
	meta := eventMeta(event)
	if meta == nil {
		return closeVerification(conn, pkg.ErrInternal)
	}
	if filter.ContestId == nil { // Global admin still checks that the event's contest exists.
		if meta.ContestId == nil {
			return writeEvent(conn, event.DTOPayload)
		}
		permissions, err = s.permissions.GetContestPermissions(ctx, *meta.ContestId, user.Id)
		if err != nil {
			return closeVerification(conn, err)
		}
	}
	own := meta.UserId != nil && *meta.UserId == user.Id
	if !permissions.ListUsersSubmissions && !permissions.GetMonitor && (!own || !permissions.ListOwnSubmissions) {
		return nil
	}
	contest, err := s.contests.GetContest(ctx, *meta.ContestId)
	if err != nil {
		return closeVerification(conn, err)
	}
	view := models.SubmissionView(&contest, meta.UserId, meta.CreatedAt, user.Id, permissions, time.Now())
	if view.Frozen && event.EventType != models.SubmissionEventCreated && event.EventType != models.SubmissionEventCompleted {
		return nil
	}
	if event.EventType == models.SubmissionEventTestStarted && !view.Details {
		return nil
	}
	// Decode the contract envelope to remove fields rather than inventing result values.
	var envelope struct {
		EventType string                     `json:"event_type"`
		Payload   map[string]json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(event.DTOPayload, &envelope); err != nil {
		return closeVerification(conn, err)
	}
	if !view.Source {
		delete(envelope.Payload, "source")
	}
	if view.Frozen {
		for _, field := range []string{"state", "score", "penalty", "time_stat", "memory_stat", "failed_test", "test_details", "source"} {
			delete(envelope.Payload, field)
		}
		envelope.Payload["frozen"] = json.RawMessage("true")
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return closeVerification(conn, err)
	}
	return writeEvent(conn, payload)
}

func (s *Observer) sendContest(ctx context.Context, filter *SubmissionsFilter, event *WrappedEvent, conn *websocket.Conn) error {
	user, permissions, err := s.currentAccess(ctx, filter)
	if err != nil {
		return closeVerification(conn, err)
	}
	if !permissions.GetContest {
		return closeVerification(conn, pkg.NoPermission)
	}
	var author models.User
	switch e := event.RawEvent.(type) {
	case *models.ContestClarificationCreatedEvent:
		author.Id = e.UserID
	case *models.ContestClarificationAnsweredEvent:
		author.Id = e.UserID
	default:
		return writeEvent(conn, event.DTOPayload)
	}
	if author.Id != user.Id && !permissions.ManageContest {
		return nil
	}
	return writeEvent(conn, event.DTOPayload)
}

func writeEvent(conn *websocket.Conn, payload []byte) error {
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("write observer event: %w", err)
	}
	return nil
}
