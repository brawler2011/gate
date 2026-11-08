package observer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
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

func (s *Observer) sessionActor(ctx context.Context) (accessmodel.User, error) {
	if err := middleware.AuthenticationError(ctx); err != nil {
		return accessmodel.User{}, err
	}
	if accessmodel.UserFromContext(ctx).IsGuest() {
		return accessmodel.Guest, nil
	}
	session, err := middleware.GetSession(ctx)
	if err != nil {
		return accessmodel.User{}, err
	}
	authenticate := s.auth.Authenticate
	if validator, ok := s.auth.(interface {
		ValidateSession(context.Context, uuid.UUID) (accessmodel.User, error)
	}); ok {
		authenticate = validator.ValidateSession
	}
	user, err := authenticate(ctx, session.ID)
	if err != nil {
		return user, err
	}
	if user.IsGuest() || user.Id != session.UserID {
		return user, pkg.ErrUnauthenticated
	}
	return user, nil
}
func (s *Observer) currentAccess(ctx context.Context, filter *SubmissionsFilter) (accessmodel.User, *accessmodel.ContestPermissions, error) {
	ctx = accessmodel.WithReadTime(ctx, time.Now().UTC())
	user, err := s.sessionActor(ctx)
	if err != nil {
		return user, nil, err
	}
	if filter.ContestId == nil {
		if user.IsGuest() {
			return user, nil, pkg.ErrUnauthenticated
		}
		if !user.IsAdmin() {
			return user, nil, pkg.NoPermission
		}
		return user, nil, nil
	}
	permissions, err := s.permissions.GetContestPermissions(ctx, *filter.ContestId, user.Id)
	return user, permissions, err
}

func submissionAccess(user accessmodel.User, permissions *accessmodel.ContestPermissions, filter *SubmissionsFilter) bool {
	if filter.ContestId == nil {
		return user.IsAdmin()
	}
	return permissions.ListUsersSubmissions || permissions.GetMonitor || (permissions.ListOwnSubmissions && (filter.UserId == nil || *filter.UserId == user.Id))
}

func closeVerification(conn eventConnection, err error) error {
	code := websocket.CloseInternalServerErr
	if errors.Is(err, pkg.NoPermission) || errors.Is(err, pkg.ErrNotFound) || errors.Is(err, pkg.ErrUnauthenticated) {
		code = websocket.ClosePolicyViolation
	}
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, http.StatusText(pkg.ToREST(err))), time.Now().Add(timeToClose))
	return err
}

func eventMeta(event *WrappedEvent) *submissionsmodel.SubmissionEventMeta {
	switch e := event.RawEvent.(type) {
	case *submissionsmodel.SubmissionCreatedEvent:
		return &e.SubmissionEventMeta
	case *submissionsmodel.SubmissionQueuedEvent:
		return &e.SubmissionEventMeta
	case *submissionsmodel.SubmissionCompilingStartedEvent:
		return &e.SubmissionEventMeta
	case *submissionsmodel.SubmissionTestingStartedEvent:
		return &e.SubmissionEventMeta
	case *submissionsmodel.SubmissionTestStartedEvent:
		return &e.SubmissionEventMeta
	case *submissionsmodel.SubmissionCompletedEvent:
		return &e.SubmissionEventMeta
	default:
		return nil
	}
}

func (s *Observer) sendSubmission(ctx context.Context, filter *SubmissionsFilter, event *WrappedEvent, conn eventConnection) error {
	ctx = accessmodel.WithReadTime(ctx, time.Now().UTC())
	if !filter.Matches(event) {
		return nil
	}
	meta := eventMeta(event)
	if meta == nil {
		return closeVerification(conn, pkg.ErrInternal)
	}
	audience, viewer, configuration := filter.audience, filter.viewer, filter.configuration
	if filter.ContestId == nil {
		// The global feed is available only to a currently authenticated global admin.
		user, _, err := s.currentAccess(ctx, filter)
		if err != nil {
			return closeVerification(conn, err)
		}
		if !user.IsAdmin() {
			return closeVerification(conn, pkg.NoPermission)
		}
		if meta.ContestId == nil {
			return writeEvent(conn, event.DTOPayload)
		}
		p, err := s.permissions.GetContestPermissions(ctx, *meta.ContestId, user.Id)
		if err != nil {
			return closeVerification(conn, err)
		}
		if !p.ManageContest {
			return closeVerification(conn, pkg.NoPermission)
		}
		c, err := s.contests.GetContest(ctx, *meta.ContestId)
		if err != nil {
			return closeVerification(conn, err)
		}
		audience, viewer, configuration = "managers", user.Id, topicConfiguration(c)
	}
	if meta.ContestId == nil || event.configuration != configuration {
		return closeVerification(conn, pkg.ErrConflict)
	}
	p := event.topics[contestTopic(*meta.ContestId, audience)]
	if own, ok := event.topics[ownTopic(*meta.ContestId, viewer)]; ok && audience != "managers" {
		p.submissions = own.submissions
	}
	for _, payload := range append(p.submissions, p.scoreboard...) {
		if err := writeEvent(conn, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Observer) sendContest(filter *SubmissionsFilter, event *WrappedEvent, conn eventConnection) error {
	p, ok := event.topics[contestTopic(*filter.ContestId, filter.audience)]
	if !ok {
		p = event.topics[ownTopic(*filter.ContestId, filter.viewer)]
	}
	for _, payload := range p.submissions {
		if err := writeEvent(conn, payload); err != nil {
			return err
		}
	}
	return nil
}

func writeEvent(conn eventConnection, payload []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return fmt.Errorf("set observer write deadline: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("write observer event: %w", err)
	}
	return nil
}
