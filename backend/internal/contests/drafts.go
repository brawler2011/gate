package contests

import (
	"context"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

const (
	maxDraftCodeSize = 64 * 1024
	maxDraftsPerUser = 50
)

type DraftTxStore interface {
	LockDraftOwner(context.Context, uuid.UUID, uuid.UUID) error
	HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error)
	GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error)
	CreateDraft(context.Context, *contestsmodel.ContestDraftCreation) (uuid.UUID, error)
	GetDraft(context.Context, uuid.UUID) (contestsmodel.ContestDraft, error)
	GetDraftsCount(context.Context, uuid.UUID, uuid.UUID) (int64, error)
	DeleteDraft(context.Context, uuid.UUID) error
}

type DraftTransactions interface {
	Within(context.Context, func(DraftTxStore) error) error
}

type DraftReads interface {
	ListDrafts(context.Context, contestsmodel.ContestDraftsFilter) (*contestsmodel.ContestDraftsList, error)
}
type DraftsUseCase struct {
	transactions DraftTransactions
	reads        DraftReads
	permissions  Permissions
}

func NewDraftsUseCase(transactions DraftTransactions, reads DraftReads, permissions Permissions) *DraftsUseCase {
	return &DraftsUseCase{transactions: transactions, reads: reads, permissions: permissions}
}

func (uc *DraftsUseCase) CreateDraft(ctx context.Context, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error) {
	if len(creation.Code) == 0 || len(creation.Code) > maxDraftCodeSize {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid draft code size")
	}
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return uuid.Nil, pkg.ErrUnauthenticated
	}
	if user.Id != creation.UserID {
		return uuid.Nil, pkg.NoPermission
	}
	var id uuid.UUID
	err := uc.transactions.Within(ctx, func(tx DraftTxStore) error {
		if err := tx.LockDraftOwner(ctx, creation.ContestID, creation.UserID); err != nil {
			return err
		}
		allowed, err := tx.HasContestPermission(ctx, creation.ContestID, user.Id, accessmodel.ActionGetContest)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}
		contest, err := tx.GetContest(ctx, creation.ContestID)
		if err != nil {
			return err
		}
		if contest.EndTime != nil && time.Now().After(*contest.EndTime) {
			manager, err := tx.HasContestPermission(ctx, contest.ID, user.Id, accessmodel.ActionManageContest)
			if err != nil {
				return err
			}
			if !manager {
				return pkg.Wrap(pkg.ErrBadInput, nil, "contest has ended; new drafts cannot be created")
			}
		}
		count, err := tx.GetDraftsCount(ctx, creation.ContestID, user.Id)
		if err != nil {
			return err
		}
		if count >= maxDraftsPerUser {
			return pkg.Wrap(pkg.ErrBadInput, nil, "maximum number of drafts reached (limit: 50)")
		}
		id, err = tx.CreateDraft(ctx, creation)
		return err
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (uc *DraftsUseCase) DeleteDraft(ctx context.Context, contestID, draftID uuid.UUID) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	return uc.transactions.Within(ctx, func(tx DraftTxStore) error {
		allowed, err := tx.HasContestPermission(ctx, contestID, user.Id, accessmodel.ActionGetContest)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}
		draft, err := tx.GetDraft(ctx, draftID)
		if err != nil {
			return err
		}
		if draft.ContestID != contestID {
			return pkg.Wrap(pkg.ErrBadInput, nil, "draft does not belong to contest")
		}
		if draft.UserID != user.Id {
			manager, err := tx.HasContestPermission(ctx, contestID, user.Id, accessmodel.ActionManageContest)
			if err != nil {
				return err
			}
			if !manager {
				return pkg.NoPermission
			}
		}
		return tx.DeleteDraft(ctx, draftID)
	})
}

func (uc *DraftsUseCase) ListDrafts(ctx context.Context, filter contestsmodel.ContestDraftsFilter) (*contestsmodel.ContestDraftsList, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}
	if user.Id != filter.UserID {
		return nil, pkg.NoPermission
	}
	var err error
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 20)
	if err != nil {
		return nil, err
	}

	allowed, err := uc.permissions.HasContestPermission(ctx, filter.ContestID, user.Id, accessmodel.ActionGetContest)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.NoPermission
	}
	return uc.reads.ListDrafts(ctx, filter)
}
