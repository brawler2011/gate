package access

import (
	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// RequireContestAudience gates a concrete server topic. Own access always also
// requires current contest visibility, including after participation revocation.
func RequireContestAudience(in m.PolicyInput, c m.ContestContext, audience string, owner uuid.UUID) error {
	if err := ApplyContestLifecycle(in, &c); err != nil {
		return err
	}
	if err := RequireContest(in, c, m.ActionGetContest); err != nil {
		return err
	}
	switch audience {
	case "public":
		return nil
	case "participants":
		return RequireContest(in, c, m.ActionListOwnSubmissions)
	case "managers":
		return RequireContest(in, c, m.ActionViewLiveScoreboard)
	case "own":
		if owner != in.Actor.Id || owner == uuid.Nil {
			return pkg.NoPermission
		}
		return RequireContest(in, c, m.ActionListOwnSubmissions)
	default:
		return pkg.NoPermission
	}
}
