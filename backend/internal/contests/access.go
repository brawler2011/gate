package contests

import (
	"context"
	"errors"

	"github.com/brawler2011/gate/backend/internal/access"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type Permissions interface {
	HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error)
}

type CommandPermissions interface {
	Permissions
	HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error)
}

func (uc *ContestsUseCase) authorizeContestProblem(ctx context.Context, contestID, problemID, packageID uuid.UUID, allowLatest bool) (uuid.UUID, error) {
	return access.AuthorizeContestProblem(ctx, uc.permissions, contestID, problemID, func() (access.PackageContext, error) {
		return uc.contestRepo.ResolveContestProblemPackage(ctx, problemID, packageID, allowLatest)
	})
}

func (uc *ContestsUseCase) GetContestScoreboard(ctx context.Context, contestID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	user := accessmodel.UserFromContext(ctx)
	userID := user.Id
	contest, err := uc.contestRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	allowed, err := uc.permissions.HasContestPermission(ctx, contestID, userID, accessmodel.ActionGetMonitor)
	if err != nil {
		return nil, err
	}
	if !allowed {
		if user.IsGuest() {
			return nil, pkg.ErrUnauthenticated
		}
		return nil, pkg.NoPermission
	}
	manager, err := uc.permissions.HasContestPermission(ctx, contestID, userID, accessmodel.ActionManageContest)
	if err != nil {
		return nil, err
	}
	if !manager && contest.StartTime != nil && accessmodel.ReadTime(ctx).Before(*contest.StartTime) {
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
	// Serialization failures require a fresh external precheck, never a replay
	// of the decision inside the transaction.
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = uc.withinContestOnce(ctx, id, action, fn)
		_, prepared := accessmodel.DecisionFromContext(ctx)
		if prepared || !errors.Is(err, pkg.ErrConflict) {
			return err
		}
	}
	return err
}

func (uc *ContestsUseCase) withinContestOnce(ctx context.Context, id uuid.UUID, action accessmodel.ContestAction, fn func(CommandTxStore) error) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	var err error
	ctx, err = uc.transactions.Prepare(ctx)
	if err != nil {
		return err
	}
	allowed, err := uc.permissions.HasContestPermission(ctx, id, user.Id, action)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.NoPermission
	}
	return uc.transactions.Within(ctx, func(tx CommandTxStore) error {
		return fn(tx)
	})
}
