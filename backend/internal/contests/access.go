package contests

import (
	"context"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type Permissions interface {
	HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error)
}

func (uc *ContestsUseCase) GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}
	if user.Id != userID {
		return nil, pkg.NoPermission
	}
	contest, err := uc.contestRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	allowed, err := uc.permissions.HasContestPermission(ctx, contestID, userID, accessmodel.ActionGetMonitor)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.NoPermission
	}
	manager := user.IsAdmin() || contest.OwnerID != nil && *contest.OwnerID == userID
	if !manager && contest.StartTime != nil && time.Now().Before(*contest.StartTime) {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "contest has not started yet")
	}
	if unfrozen {
		allowed, err := uc.permissions.HasContestPermission(ctx, contestID, userID, accessmodel.ActionManageContest)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, pkg.NoPermission
		}
	}
	return uc.ProjectScoreboard(ctx, contestID, unfrozen)
}

func (uc *ContestsUseCase) withinContest(ctx context.Context, id uuid.UUID, action accessmodel.ContestAction, fn func(CommandTxStore) error) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	return uc.transactions.Within(ctx, func(tx CommandTxStore) error {
		allowed, err := tx.HasContestPermission(ctx, id, user.Id, action)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}
		return fn(tx)
	})
}
