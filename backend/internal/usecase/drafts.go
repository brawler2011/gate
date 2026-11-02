package usecase

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

const (
	maxDraftCodeSize = 64 * 1024 // 64 KB
	maxDraftsPerUser = 50
)

type DraftsUseCase struct {
	draftsRepo    interfaces.DraftsRepo
	contestsUC    interfaces.ContestsUC
	permissionsUC interfaces.PermissionsUC
	transactor    interfaces.Transactor
}

func NewDraftsUseCase(
	draftsRepo interfaces.DraftsRepo,
	contestsUC interfaces.ContestsUC,
	permissionsUC interfaces.PermissionsUC,
	transactor interfaces.Transactor,
) *DraftsUseCase {
	return &DraftsUseCase{
		draftsRepo:    draftsRepo,
		contestsUC:    contestsUC,
		permissionsUC: permissionsUC,
		transactor:    transactor,
	}
}

func (uc *DraftsUseCase) CreateDraft(ctx context.Context, creation *models.ContestDraftCreation) (uuid.UUID, error) {
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
