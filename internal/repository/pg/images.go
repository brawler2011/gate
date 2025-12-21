package pg

import (
	"context"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/repository/pg/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImagesRepo struct {
	queries *sqlc.Queries
}

func NewImagesRepo(p *pgxpool.Pool) *ImagesRepo {
	return &ImagesRepo{
		queries: sqlc.New(p),
	}
}

func (r *ImagesRepo) WithTx(tx pgx.Tx) interfaces.ImagesRepo {
	return &ImagesRepo{
		queries: sqlc.New(tx),
	}
}

func (r *ImagesRepo) CreateImage(ctx context.Context, params models.CreateImageParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	err := r.queries.CreateImage(ctx, sqlc.CreateImageParams{
		ID:    params.Id,
		Image: params.Image,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *ImagesRepo) GetImageById(ctx context.Context, id uuid.UUID) (models.ImageRecord, error) {
	image, err := r.queries.GetImageById(ctx, id)
	if err != nil {
		return models.ImageRecord{}, pkg.HandlePgErr(err)
	}
	return models.ImageRecord{
		Id:        image.ID,
		Image:     image.Image,
		CreatedAt: image.CreatedAt,
		UpdatedAt: image.UpdatedAt,
	}, nil
}

func (r *ImagesRepo) DeleteImage(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteImage(ctx, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}
