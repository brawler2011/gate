package interfaces

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

type PackagesRepo interface {
	CreatePackage(ctx context.Context, params *models.CreatePackageParams) (models.ProblemPackage, error)
	ListPackages(ctx context.Context, problemID uuid.UUID, limit, offset int32) ([]models.ProblemPackage, error)
	GetReadyPackage(ctx context.Context, problemID uuid.UUID) (models.ProblemPackage, error)
	UpdatePackageStatus(ctx context.Context, params *models.UpdatePackageStatusParams) error
}
