package contests

import (
	"context"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

const (
	maxDraftCodeSize = 64 * 1024 // 64 KB
	maxDraftsPerUser = 50
)

type DraftsUseCase struct {
	draftsRepo DraftsRepo
	contestsUC ContestsUC
}

func NewDraftsUseCase(
	draftsRepo DraftsRepo,
	contestsUC ContestsUC) *DraftsUseCase {
	return &DraftsUseCase{
		draftsRepo: draftsRepo,
		contestsUC: contestsUC,
	}
}

func (uc *DraftsUseCase) CreateDraft(ctx context.Context, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error) {
	codeSize := len(creation.Code)
	if codeSize == 0 || codeSize > maxDraftCodeSize {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid draft code size")
	}

	if uc.contestsUC != nil && creation.ContestID != uuid.Nil {
		_, err := uc.contestsUC.GetContest(ctx, creation.ContestID)
		if err != nil {
			return uuid.Nil, err
		}
	}

	count, err := uc.draftsRepo.GetDraftsCount(ctx, creation.ContestID, creation.UserID)
	if err != nil {
		return uuid.Nil, err
	}
	if count >= maxDraftsPerUser {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "maximum number of drafts reached (limit: 50)")
	}

	return uc.draftsRepo.CreateDraft(ctx, creation)
}

func (uc *DraftsUseCase) DeleteDraft(ctx context.Context, draftID, userID uuid.UUID, isManager bool) error {
	draft, err := uc.draftsRepo.GetDraft(ctx, draftID)
	if err != nil {
		return err
	}

	if !isManager && draft.UserID != userID {
		return pkg.Wrap(pkg.NoPermission, nil, "cannot delete other user's draft")
	}

	return uc.draftsRepo.DeleteDraft(ctx, draftID)
}
