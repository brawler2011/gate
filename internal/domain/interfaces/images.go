package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ImagesRepo interface {
	WithTx(tx pgx.Tx) ImagesRepo // FIXME: layers violation
	CreateImage(ctx context.Context, params models.CreateImageParams) error
	GetImageById(ctx context.Context, id uuid.UUID) (models.ImageRecord, error)
	DeleteImage(ctx context.Context, id uuid.UUID) error
}
