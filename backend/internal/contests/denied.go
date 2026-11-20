package contests

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type contestPermissionChecker interface {
	HasContestPermission(ctx context.Context, contestID, userID uuid.UUID, action accessmodel.ContestAction) (bool, error)
}

// deniedContest hides a contest the caller cannot see; only a visible one is forbidden.
func deniedContest(ctx context.Context, permissions contestPermissionChecker, id uuid.UUID, action accessmodel.ContestAction) error {
	if action != accessmodel.ActionGetContest {
		visible, err := permissions.HasContestPermission(ctx, id, accessmodel.UserFromContext(ctx).Id, accessmodel.ActionGetContest)
		if err != nil {
			return err
		}
		if visible {
			return pkg.ErrForbidden
		}
	}
	return pkg.ErrNotFound
}
