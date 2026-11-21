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
	CanContest(context.Context, uuid.UUID, accessmodel.ContestAction) (bool, error)
}

type CommandPermissions interface {
	Permissions
	CanProblem(context.Context, uuid.UUID, accessmodel.ProblemAction) (bool, error)
}

func (uc *ContestsUseCase) authorizeContestProblem(ctx context.Context, contestID, problemID, packageID uuid.UUID, allowLatest bool) (uuid.UUID, error) {
	return access.AuthorizeContestProblem(ctx, uc.permissions, contestID, problemID, func() (access.PackageContext, error) {
		return uc.contestRepo.ResolveContestProblemPackage(ctx, problemID, packageID, allowLatest)
	})
}

func (uc *ContestsUseCase) GetContestScoreboard(ctx context.Context, contestID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	contest, err := uc.contestRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	if err := access.Contest(ctx, uc.permissions, contestID, accessmodel.ActionGetMonitor); err != nil {
		return nil, err
	}
	manager, err := uc.permissions.CanContest(ctx, contestID, accessmodel.ActionManageContest)
	if err != nil {
		return nil, err
	}
	if !manager && contest.StartTime != nil && accessmodel.ReadTime(ctx).Before(*contest.StartTime) {
		return nil, pkg.Wrap(pkg.ErrContestNotStarted, nil, "contest has not started yet")
	}
	if unfrozen {
		if err := access.Contest(ctx, uc.permissions, contestID, accessmodel.ActionManageContest); err != nil {
			return nil, err
		}
	}
	return uc.ProjectScoreboard(ctx, contestID, unfrozen)
}

func (uc *ContestsUseCase) withinContest(ctx context.Context, id uuid.UUID, action accessmodel.ContestAction, fn func(CommandTxStore) error) error {
	// Serialization failures require a fresh external precheck.
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = uc.withinContestOnce(ctx, id, action, fn)
		_, prepared := accessmodel.DecisionFromContext(ctx)
		if prepared || !errors.Is(err, pkg.ErrConcurrentUpdate) {
			return err
		}
	}
	return err
}

func (uc *ContestsUseCase) withinContestOnce(ctx context.Context, id uuid.UUID, action accessmodel.ContestAction, fn func(CommandTxStore) error) error {
	var err error
	ctx, err = uc.transactions.Prepare(ctx)
	if err != nil {
		return err
	}
	if err := access.Contest(ctx, uc.permissions, id, action); err != nil {
		return err
	}
	return uc.transactions.Within(ctx, func(tx CommandTxStore) error {
		return fn(tx)
	})
}
