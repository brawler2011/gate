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

func (uc *ContestsUseCase) ListPublicContests(ctx context.Context, filter contestsmodel.PublicContestsFilter) (*contestsmodel.ContestsList, error) {
	return uc.contestRepo.ListPublicContests(ctx, filter)
}

func (uc *ContestsUseCase) ListWorkshopContests(ctx context.Context, filter contestsmodel.WorkshopContestsFilter) (*contestsmodel.ContestsList, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, filter.UserId); err != nil {
		return nil, err
	}
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return uc.contestRepo.ListWorkshopContests(ctx, filter)
}

func (uc *ContestsUseCase) ListUserContests(ctx context.Context, filter contestsmodel.UserContestsFilter) (*contestsmodel.ContestsList, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, filter.UserId); err != nil {
		return nil, err
	}
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return uc.contestRepo.ListUserContests(ctx, filter)
}

func (uc *ContestsUseCase) ListAdminContests(ctx context.Context, filter contestsmodel.AdminContestsFilter) (*contestsmodel.ContestsList, error) {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return uc.contestRepo.ListAdminContests(ctx, filter)
}

func (uc *ContestsUseCase) ListOrganizationContests(ctx context.Context, login, search string, page, size int32) (*contestsmodel.ContestsList, error) {
	var err error
	page, size, err = paging.Normalize(page, size, 50)
	if err != nil {
		return nil, err
	}
	return uc.contestRepo.ListOrganizationContests(ctx, login, search, page, size)
}

func (uc *ContestsUseCase) authorizeRoster(ctx context.Context, contest uuid.UUID) error {
	// Participants and scoreboard viewers see the roster.
	allowed, err := uc.permissions.CanContest(ctx, contest, accessmodel.ActionListOwnSubmissions)
	if err != nil || allowed {
		return err
	}
	return access.Contest(ctx, uc.permissions, contest, accessmodel.ActionGetMonitor)
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

func (uc *ContestsUseCase) ListDashboardContests(ctx context.Context, user uuid.UUID, limit int32) ([]contestsmodel.DashboardContest, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, user); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, pkg.ErrInvalid
	}
	return uc.contestRepo.ListDashboardContests(ctx, user, limit)
}
