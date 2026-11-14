package contests

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type AnnouncementReads interface {
	ListAnnouncements(context.Context, uuid.UUID, int32, int32) (*contestsmodel.ContestAnnouncementsList, error)
}

type ClarificationReads interface {
	ListClarificationsForUser(context.Context, uuid.UUID, uuid.UUID, int32, int32) (*contestsmodel.ContestClarificationsList, error)
	ListClarificationsForModerator(context.Context, uuid.UUID, *contestsmodel.ContestClarificationsFilter) (*contestsmodel.ContestClarificationsList, error)
}

func (uc *AnnouncementsUseCase) ListAnnouncements(ctx context.Context, contest uuid.UUID, page, size int32) (*contestsmodel.ContestAnnouncementsList, error) {
	if err := paging.Validate(page, size); err != nil {
		return nil, err
	}
	allowed, err := uc.permissions.HasContestPermission(ctx, contest, accessmodel.UserFromContext(ctx).Id, accessmodel.ActionGetContest)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.NoPermission
	}
	return uc.reads.ListAnnouncements(ctx, contest, page, size)
}

func (uc *ClarificationsUseCase) ListClarifications(ctx context.Context, contest uuid.UUID, filter *contestsmodel.ContestClarificationsFilter) (*contestsmodel.ContestClarificationsList, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}
	if err := paging.Validate(filter.Page, filter.PageSize); err != nil {
		return nil, err
	}
	allowed, err := uc.permissions.HasContestPermission(ctx, contest, user.Id, accessmodel.ActionGetContest)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.NoPermission
	}
	moderator, err := uc.permissions.HasContestPermission(ctx, contest, user.Id, accessmodel.ActionManageContest)
	if err != nil {
		return nil, err
	}
	if moderator {
		return uc.reads.ListClarificationsForModerator(ctx, contest, filter)
	}
	return uc.reads.ListClarificationsForUser(ctx, contest, user.Id, filter.Page, filter.PageSize)
}
