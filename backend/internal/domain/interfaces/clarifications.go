package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ClarificationsRepo interface {
	WithTx(tx pgx.Tx) ClarificationsRepo
	CreateClarification(ctx context.Context, input *models.CreateContestClarificationInput) (*models.ContestClarification, error)
	GetClarificationByID(ctx context.Context, id uuid.UUID) (*models.ContestClarification, error)
	AnswerClarification(ctx context.Context, id, contestID, answeredBy uuid.UUID, answer string) (*models.ContestClarification, error)
}
