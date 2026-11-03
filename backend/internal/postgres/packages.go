package postgres

import (
	"context"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
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

func (r *PackagesRepo) CreatePackage(ctx context.Context, params *problemsmodel.CreatePackageParams) (problemsmodel.ProblemPackage, error) {
	row, err := r.queries.CreateProblemPackage(ctx, sqlc.CreateProblemPackageParams{
		ID:             params.ID,
		ProblemID:      params.ProblemID,
		OrganizationID: params.OrganizationID,
		PackageHash:    params.PackageHash,
		Status:         sqlc.PackageStatus(params.Status),
	})
	if err != nil {
		return problemsmodel.ProblemPackage{}, HandlePgErr(err)
	}
	return mapPackage(row), nil
}

func (r *PackagesRepo) ListPackages(ctx context.Context, problemID uuid.UUID, limit, offset int32) ([]problemsmodel.ProblemPackage, error) {
	rows, err := r.queries.ListProblemPackages(ctx, sqlc.ListProblemPackagesParams{
		ProblemID: problemID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	result := make([]problemsmodel.ProblemPackage, len(rows))
	for i, row := range rows {
		result[i] = mapPackage(row)
	}
	return result, nil
}

func (r *PackagesRepo) GetReadyPackage(ctx context.Context, problemID uuid.UUID) (problemsmodel.ProblemPackage, error) {
	row, err := r.queries.GetReadyPackage(ctx, problemID)
	if err != nil {
		return problemsmodel.ProblemPackage{}, HandlePgErr(err)
	}
	return mapPackage(row), nil
}

func (r *PackagesRepo) GetPackageByID(ctx context.Context, id uuid.UUID) (problemsmodel.ProblemPackage, error) {
	row, err := r.queries.GetProblemPackageByID(ctx, id)
	if err != nil {
		return problemsmodel.ProblemPackage{}, HandlePgErr(err)
	}
	return mapPackage(row), nil
}

func (r *PackagesRepo) GetPackageByVersion(ctx context.Context, problemID uuid.UUID, version int32) (problemsmodel.ProblemPackage, error) {
	row, err := r.queries.GetProblemPackageByVersion(ctx, sqlc.GetProblemPackageByVersionParams{
		ProblemID: problemID,
		Version:   version,
	})
	if err != nil {
		return problemsmodel.ProblemPackage{}, HandlePgErr(err)
	}
	return mapPackage(row), nil
}

func mapPackage(p sqlc.ProblemPackage) problemsmodel.ProblemPackage {
	pkg := problemsmodel.ProblemPackage{
		ID:             p.ID,
		ProblemID:      p.ProblemID,
		OrganizationID: p.OrganizationID,
		Version:        p.Version,
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
