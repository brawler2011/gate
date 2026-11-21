package contests

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// usableProblemPackage checks the source problem inside the contest command and resolves the package to pin.
func usableProblemPackage(ctx context.Context, tx CommandTxStore, problemID, packageID uuid.UUID, allowLatest bool) (uuid.UUID, error) {
	if _, err := access.Problem(ctx, tx, problemID, accessmodel.ProblemUse); err != nil {
		return uuid.Nil, err
	}
	pack, err := tx.ResolveContestProblemPackage(ctx, problemID, packageID, allowLatest)
	if err != nil {
		return uuid.Nil, err
	}
	if pack.ID == uuid.Nil || pack.ProblemID != problemID {
		return uuid.Nil, pkg.ErrInvalid
	}
	return pack.ID, nil
}

func (uc *ContestsUseCase) GetContestScoreboard(ctx context.Context, contestID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	a, err := access.Contest(ctx, uc.access, contestID, accessmodel.ContestView)
	if err != nil {
		return nil, err
	}
	if !a.Can(accessmodel.ContestViewScoreboard) {
		start := a.Facts.Contest.StartTime
		if start != nil && a.Facts.Now.Before(*start) {
			return nil, pkg.Wrap(pkg.ErrContestNotStarted, nil, "contest has not started yet")
		}
		return nil, pkg.ErrForbidden
	}
	if unfrozen && !a.Can(accessmodel.ContestManage) {
		return nil, pkg.ErrForbidden
	}
	return uc.ProjectScoreboard(ctx, contestID, unfrozen)
}
