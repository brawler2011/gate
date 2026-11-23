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

const (
	maxDraftCodeSize = 64 * 1024
	maxDraftsPerUser = 50
)

type DraftTxStore interface {
	access.Loader
	LockDraftOwner(context.Context, uuid.UUID, uuid.UUID) error
	CreateDraft(ctx context.Context, userID uuid.UUID, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error)
	GetDraft(context.Context, uuid.UUID) (contestsmodel.ContestDraft, error)
	GetDraftsCount(context.Context, uuid.UUID, uuid.UUID) (int64, error)
	DeleteDraft(context.Context, uuid.UUID) error
}

type DraftTransactions interface {
	Within(context.Context, func(DraftTxStore) error) error
}

type DraftReads interface {
	ListDrafts(ctx context.Context, userID uuid.UUID, filter contestsmodel.ContestDraftsFilter) (*contestsmodel.ContestDraftsList, error)
}
type DraftsUseCase struct {
	transactions DraftTransactions
	reads        DraftReads
	access       access.Loader
}

func NewDraftsUseCase(transactions DraftTransactions, reads DraftReads, loader access.Loader) *DraftsUseCase {
	return &DraftsUseCase{transactions: transactions, reads: reads, access: loader}
}

func (uc *DraftsUseCase) CreateDraft(ctx context.Context, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error) {
	if len(creation.Code) == 0 {
		return uuid.Nil, pkg.Wrap(pkg.ErrRequired, nil, "draft code is empty")
	}
	if len(creation.Code) > maxDraftCodeSize {
		return uuid.Nil, pkg.Invalid(pkg.ErrTooLarge, "draft code is too large", map[string]any{"max": maxDraftCodeSize})
	}
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = access.WithinContest(ctx, uc.transactions, creation.ContestID, accessmodel.ContestView, func(tx DraftTxStore, a access.ContestAccess) error {
		if err := draftsEnabled(a); err != nil {
			return err
		}
		if err := tx.LockDraftOwner(ctx, creation.ContestID, user.Id); err != nil {
			return err
		}
		end := a.Facts.Contest.EndTime
		if end != nil && !a.Facts.Now.Before(*end) && !a.Can(accessmodel.ContestManage) {
			return pkg.Wrap(pkg.ErrContestEnded, nil, "contest has ended; new drafts cannot be created")
		}
		count, err := tx.GetDraftsCount(ctx, creation.ContestID, user.Id)
		if err != nil {
			return err
		}
		if count >= maxDraftsPerUser {
			return pkg.Invalid(pkg.ErrLimitReached, "maximum number of drafts reached", map[string]any{"limit": maxDraftsPerUser})
		}
		id, err = tx.CreateDraft(ctx, user.Id, creation)
		return err
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (uc *DraftsUseCase) DeleteDraft(ctx context.Context, contestID, draftID uuid.UUID) error {
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return err
	}
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestView, func(tx DraftTxStore, a access.ContestAccess) error {
		if err := draftsEnabled(a); err != nil {
			return err
		}
		draft, err := tx.GetDraft(ctx, draftID)
		if err != nil {
			return err
		}
		if draft.ContestID != contestID {
			return pkg.Wrap(pkg.ErrNotFound, nil, "draft does not belong to contest")
		}
		if draft.UserID != user.Id && !a.Can(accessmodel.ContestManage) {
			return pkg.ErrForbidden
		}
		return tx.DeleteDraft(ctx, draftID)
	})
}

func (uc *DraftsUseCase) ListDrafts(ctx context.Context, filter contestsmodel.ContestDraftsFilter) (*contestsmodel.ContestDraftsList, error) {
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	filter.Page, filter.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 20)
	if err != nil {
		return nil, err
	}

	a, err := access.Contest(ctx, uc.access, filter.ContestID, accessmodel.ContestView)
	if err != nil {
		return nil, err
	}
	if err := draftsEnabled(a); err != nil {
		return nil, err
	}
	return uc.reads.ListDrafts(ctx, user.Id, filter)
}

// Disabling drafts hides them from everyone, managers included, and keeps them stored.
func draftsEnabled(a access.ContestAccess) error {
	if !a.Facts.Contest.Settings.EnableDrafts {
		return pkg.Wrap(pkg.ErrDraftsDisabled, nil, "drafts are disabled for this contest")
	}
	return nil
}
