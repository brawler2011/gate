package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DraftsRepo interface {
	CreateDraft(ctx context.Context, creation *models.ContestDraftCreation) (uuid.UUID, error)
	GetDraft(ctx context.Context, id uuid.UUID) (models.ContestDraft, error)
	GetDraftsCount(ctx context.Context, contestID, userID uuid.UUID) (int64, error)
	DeleteDraft(ctx context.Context, id uuid.UUID) error
	WithTx(tx pgx.Tx) DraftsRepo
}

type DraftsUC interface {
	CreateDraft(ctx context.Context, creation *models.ContestDraftCreation) (uuid.UUID, error)
	DeleteDraft(ctx context.Context, draftID, userID uuid.UUID, isManager bool) error
}
