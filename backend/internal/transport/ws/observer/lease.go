package observer

import (
	"context"
	"fmt"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type RevisionReader interface {
	AccessRevision(context.Context) (int64, error)
}
type audienceAuthorizer interface {
	AuthorizeContestAudience(context.Context, uuid.UUID, uuid.UUID, string, uuid.UUID) error
}

// admit fixes the audience and security revision before history is selected.
// Every delivery checks the lease, including replay; idle sessions are checked
// too. Revision read failure closes subscriptions, just like invalidation loss.
func (s *Observer) admit(ctx context.Context, filter *SubmissionsFilter, contestEvents bool) (func() error, error) {
	signature := func() (string, error) {
		ctx := m.WithReadTime(ctx, time.Now().UTC())
		var revision int64
		if s.revisions != nil {
			var err error
			revision, err = s.revisions.AccessRevision(ctx)
			if err != nil {
				return "", err
			}
		}
		user, p, err := s.currentAccess(ctx, filter)
		if err != nil {
			return "", err
		}
		if p == nil {
			if !user.IsAdmin() {
				return "", pkg.NoPermission
			}
			return fmt.Sprint(revision, user), nil
		}
		if contestEvents {
			if !p.GetContest {
				return "", pkg.NoPermission
			}
		} else if !submissionAccess(user, p, filter) {
			return "", pkg.NoPermission
		}
		audience := "public"
		if p.ListOwnSubmissions {
			audience = "participants"
		}
		if p.ManageContest {
			audience = "managers"
		}
		if authorize, ok := s.permissions.(audienceAuthorizer); ok {
			if err := authorize.AuthorizeContestAudience(ctx, *filter.ContestId, user.Id, audience, uuid.Nil); err != nil {
				return "", err
			}
		}
		topics := []string{}
		if contestEvents || p.ListUsersSubmissions || p.GetMonitor {
			topics = append(topics, contestTopic(*filter.ContestId, audience))
		}
		if p.ListOwnSubmissions && !p.ManageContest {
			if authorize, ok := s.permissions.(audienceAuthorizer); ok {
				if err := authorize.AuthorizeContestAudience(ctx, *filter.ContestId, user.Id, "own", user.Id); err != nil {
					return "", err
				}
			}
			topics = append(topics, ownTopic(*filter.ContestId, user.Id))
		}
		if filter.UserId != nil && *filter.UserId != user.Id && !p.ListUsersSubmissions && !p.GetMonitor {
			return "", pkg.NoPermission
		}
		c, err := s.contests.GetContest(ctx, *filter.ContestId)
		if err != nil {
			return "", err
		}
		// Bind only on admission; verification never changes queued audiences.
		if filter.Topics == nil {
			filter.Topics = topics
			filter.audience = audience
			filter.viewer = user.Id
			filter.configuration = topicConfiguration(c)
		}
		if s.revisions != nil {
			current, err := s.revisions.AccessRevision(ctx)
			if err != nil {
				return "", err
			}
			if current != revision {
				return "", pkg.ErrConflict
			}
		}
		// Session revocation can occur while facts are being loaded without changing
		// resource revisions. Recheck credentials at the delivery decision boundary.
		currentActor, err := s.sessionActor(ctx)
		if err != nil {
			return "", err
		}
		if currentActor.Id != user.Id || currentActor.Role != user.Role {
			return "", pkg.ErrUnauthenticated
		}
		return fmt.Sprintf("%d/%s/%v/%s", revision, user.Id, *p, topicConfiguration(c)), nil
	}
	initial, err := signature()
	if err != nil {
		return nil, err
	}
	verify := func() error {
		current, err := signature()
		if err != nil {
			return err
		}
		if initial != current {
			return pkg.ErrConflict
		}
		return nil
	}
	return verify, nil
}
