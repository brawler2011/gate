package observer

import (
	"context"
	"fmt"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
)

// admit fixes the audience before selecting history; every delivery and idle check recomputes it,
// so a revoked grant closes the subscription at the next event.
func (s *Observer) admit(ctx context.Context, filter *SubmissionsFilter, contestEvents bool) (func() error, error) {
	signature := func() (string, error) {
		user, a, err := s.currentAccess(ctx, filter)
		if err != nil {
			return "", err
		}
		if a == nil {
			return fmt.Sprint(user.Id, user.Role), nil
		}
		can := a.Can
		if contestEvents {
			if !can(accessmodel.ContestView) {
				return "", pkg.ErrForbidden
			}
		} else if !submissionAccess(user, a, filter) {
			return "", pkg.ErrForbidden
		}
		audience := "public"
		if a.Participates() {
			audience = "participants"
		}
		if can(accessmodel.ContestManage) {
			audience = "managers"
		}
		topics := []string{}
		if contestEvents || can(accessmodel.ContestListSubmissions) || can(accessmodel.ContestViewScoreboard) {
			topics = append(topics, contestTopic(*filter.ContestId, audience))
		}
		if can(accessmodel.ContestListOwnSubmissions) && !can(accessmodel.ContestManage) {
			topics = append(topics, ownTopic(*filter.ContestId, user.Id))
		}
		if filter.UserId != nil && *filter.UserId != user.Id && !can(accessmodel.ContestListSubmissions) && !can(accessmodel.ContestViewScoreboard) {
			return "", pkg.ErrForbidden
		}
		// Events are projected from the contest reader, so the lease must bind the same configuration.
		c, err := s.contests.GetContest(ctx, *filter.ContestId)
		if err != nil {
			return "", err
		}
		configuration := topicConfiguration(c)
		// Bind only on admission; verification never changes queued audiences.
		if filter.Topics == nil {
			filter.Topics = topics
			filter.audience = audience
			filter.viewer = user.Id
			filter.configuration = configuration
		}
		// Recheck credentials after the grants: a session can be revoked while they load.
		current, err := s.sessionActor(ctx)
		if err != nil {
			return "", err
		}
		if current.Id != user.Id || current.Role != user.Role {
			return "", pkg.ErrUnauthenticated
		}
		return fmt.Sprintf("%s/%s/%s/%v", user.Id, audience, configuration, topics), nil
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
