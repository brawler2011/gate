package pg

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PackagesRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewPackagesRepo(pool *pgxpool.Pool) *PackagesRepo {
	return &PackagesRepo{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *PackagesRepo) CreatePackage(ctx context.Context, params *models.CreatePackageParams) (models.ProblemPackage, error) {
	row, err := r.queries.CreateProblemPackage(ctx, sqlc.CreateProblemPackageParams{
		ID:             params.ID,
		ProblemID:      params.ProblemID,
		OrganizationID: params.OrganizationID,
		GitCommitHash:  params.GitCommitHash,
		PackageHash:    params.PackageHash,
		Status:         sqlc.PackageStatus(params.Status),
	})
	if err != nil {
		return models.ProblemPackage{}, HandlePgErr(err)
	}
	return mapPackage(row), nil
}

func (r *PackagesRepo) ListPackages(ctx context.Context, problemID uuid.UUID, limit, offset int32) ([]models.ProblemPackage, error) {
	rows, err := r.queries.ListProblemPackages(ctx, sqlc.ListProblemPackagesParams{
		ProblemID: problemID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	result := make([]models.ProblemPackage, len(rows))
	for i, row := range rows {
		result[i] = mapPackage(row)
	}
	return result, nil
}

func (r *PackagesRepo) GetReadyPackage(ctx context.Context, problemID uuid.UUID) (models.ProblemPackage, error) {
	row, err := r.queries.GetReadyPackage(ctx, problemID)
	if err != nil {
		return models.ProblemPackage{}, HandlePgErr(err)
	}
	return mapPackage(row), nil
}

func (r *PackagesRepo) UpdatePackageStatus(ctx context.Context, params *models.UpdatePackageStatusParams) error {
	err := r.queries.UpdatePackageStatus(ctx, sqlc.UpdatePackageStatusParams{
		ID:       params.ID,
		Status:   sqlc.PackageStatus(params.Status),
		Url:      params.URL,
		BuildLog: params.BuildLog,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func mapPackage(p sqlc.ProblemPackage) models.ProblemPackage {
	pkg := models.ProblemPackage{
		ID:             p.ID,
		ProblemID:      p.ProblemID,
		OrganizationID: p.OrganizationID,
		Version:        p.Version,
		GitCommitHash:  p.GitCommitHash,
		PackageHash:    p.PackageHash,
		URL:            p.Url,
		Status:         string(p.Status),
		BuildLog:       p.BuildLog,
		CreatedAt:      p.CreatedAt,
	}
	if p.CompiledAt.Valid {
		t := p.CompiledAt.Time
		pkg.CompiledAt = &t
	}
	return pkg
}

func init() {
	// Compile-time check that PackagesRepo implements interfaces.PackagesRepo
	var _ interface {
		CreatePackage(context.Context, *models.CreatePackageParams) (models.ProblemPackage, error)
		ListPackages(context.Context, uuid.UUID, int32, int32) ([]models.ProblemPackage, error)
		GetReadyPackage(context.Context, uuid.UUID) (models.ProblemPackage, error)
		UpdatePackageStatus(context.Context, *models.UpdatePackageStatusParams) error
	} = (*PackagesRepo)(nil)
}

