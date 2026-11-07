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

type Reads interface {
	GetContestByOrgLoginAndContestLogin(context.Context, string, string) (contestsmodel.Contest, error)
	GetContestTeams(context.Context, uuid.UUID) ([]contestsmodel.ContestTeam, error)
	ListPublicContests(context.Context, contestsmodel.PublicContestsFilter) (*contestsmodel.ContestsList, error)
	ListWorkshopContests(context.Context, contestsmodel.WorkshopContestsFilter) (*contestsmodel.ContestsList, error)
	ListUserContests(context.Context, contestsmodel.UserContestsFilter) (*contestsmodel.ContestsList, error)
	ListAdminContests(context.Context, contestsmodel.AdminContestsFilter) (*contestsmodel.ContestsList, error)
	ListOrganizationContests(context.Context, string, string, int32, int32) (*contestsmodel.ContestsList, error)
	ListContestMembers(context.Context, contestsmodel.ParticipantsFilter) (*contestsmodel.ContestMembersList, error)
	ListDashboardContests(context.Context, uuid.UUID, int32) ([]contestsmodel.DashboardContest, error)
}

// Reader applies access checks and pagination before delegating contest reads.
type Reader struct {
	Reads
	permissions Permissions
}

func NewReader(reads Reads, permissions Permissions) *Reader {
	return &Reader{Reads: reads, permissions: permissions}
}

func (r *Reader) ListPublicContests(ctx context.Context, filter contestsmodel.PublicContestsFilter) (*contestsmodel.ContestsList, error) {
	if err := access.CheckReadActor(ctx); err != nil {
		return nil, err
	}
	return r.Reads.ListPublicContests(ctx, filter)
}

func (r *Reader) ListWorkshopContests(ctx context.Context, filter contestsmodel.WorkshopContestsFilter) (*contestsmodel.ContestsList, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, filter.UserId); err != nil {
		return nil, err
	}
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return r.Reads.ListWorkshopContests(ctx, filter)
}

func (r *Reader) ListUserContests(ctx context.Context, filter contestsmodel.UserContestsFilter) (*contestsmodel.ContestsList, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, filter.UserId); err != nil {
		return nil, err
	}
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return r.Reads.ListUserContests(ctx, filter)
}

func (r *Reader) ListAdminContests(ctx context.Context, filter contestsmodel.AdminContestsFilter) (*contestsmodel.ContestsList, error) {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return r.Reads.ListAdminContests(ctx, filter)
}

func (r *Reader) ListOrganizationContests(ctx context.Context, login, search string, page, size int32) (*contestsmodel.ContestsList, error) {
	if err := access.CheckReadActor(ctx); err != nil {
		return nil, err
	}
	var err error
	page, size, err = paging.Normalize(page, size, 50)
	if err != nil {
		return nil, err
	}
	return r.Reads.ListOrganizationContests(ctx, login, search, page, size)
}

func (r *Reader) authorizeRoster(ctx context.Context, contest uuid.UUID) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	for _, action := range []accessmodel.ContestAction{accessmodel.ActionGetMonitor, accessmodel.ActionListOwnSubmissions} {
		allowed, err := r.permissions.HasContestPermission(ctx, contest, user.Id, action)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
	}
	return pkg.NoPermission
}

func (r *Reader) GetContestTeams(ctx context.Context, contest uuid.UUID) ([]contestsmodel.ContestTeam, error) {
	if err := r.authorizeRoster(ctx, contest); err != nil {
		return nil, err
	}
	return r.Reads.GetContestTeams(ctx, contest)
}

func (r *Reader) ListContestMembers(ctx context.Context, filter contestsmodel.ParticipantsFilter) (*contestsmodel.ContestMembersList, error) {
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	if err := r.authorizeRoster(ctx, filter.ContestId); err != nil {
		return nil, err
	}
	return r.Reads.ListContestMembers(ctx, filter)
}

func (r *Reader) ListDashboardContests(ctx context.Context, user uuid.UUID, limit int32) ([]contestsmodel.DashboardContest, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, user); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, pkg.ErrBadInput
	}
	return r.Reads.ListDashboardContests(ctx, user, limit)
}
