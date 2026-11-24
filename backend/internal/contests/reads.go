package contests

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Lists contests visible to the caller; a team filter also requires seeing the team.
func (uc *ContestsUseCase) ListContests(ctx context.Context, filter contestsmodel.ListContestsFilter) (*contestsmodel.ContestsList, error) {
	if filter.TeamID != nil {
		if _, err := access.Team(ctx, uc.access, *filter.TeamID, accessmodel.TeamView); err != nil {
			return nil, err
		}
	}
	return uc.contestRepo.ListContests(ctx, filter)
}

// Competitors and scoreboard viewers see the roster.
func (uc *ContestsUseCase) authorizeRoster(ctx context.Context, contest uuid.UUID) error {
	a, err := access.Contest(ctx, uc.access, contest, accessmodel.ContestView)
	if err != nil {
		return err
	}
	if !a.Can(accessmodel.ContestListOwnSubmissions) && !a.Can(accessmodel.ContestViewScoreboard) {
		return pkg.ErrForbidden
	}
	return nil
}

func (uc *ContestsUseCase) GetContestTeams(ctx context.Context, contest uuid.UUID) ([]contestsmodel.ContestTeam, error) {
	if err := uc.authorizeRoster(ctx, contest); err != nil {
		return nil, err
	}
	return uc.contestRepo.GetContestTeams(ctx, contest)
}

func (uc *ContestsUseCase) ListContestMembers(ctx context.Context, filter contestsmodel.ParticipantsFilter) (*contestsmodel.ContestMembersList, error) {
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	if err := uc.authorizeRoster(ctx, filter.ContestId); err != nil {
		return nil, err
	}
	return uc.contestRepo.ListContestMembers(ctx, filter)
}
