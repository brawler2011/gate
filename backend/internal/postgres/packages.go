package postgres

import (
	"context"
	"encoding/json"

	paging "github.com/brawler2011/gate/backend/internal/page"

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

func (r *PackagesRepo) ListPackages(ctx context.Context, problemID uuid.UUID, request paging.Request) ([]problemsmodel.ProblemPackage, int32, error) {
	request, err := request.Normalize()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.queries.ListProblemPackages(ctx, sqlc.ListProblemPackagesParams{ProblemID: problemID, PageSize: request.PageSize, PageOffset: request.Offset()})
	if err != nil {
		return nil, 0, dbError("list packages", err)
	}
	result := make([]problemsmodel.ProblemPackage, len(rows))
	for i, row := range rows {
		if result[i], err = mapPackage(row); err != nil {
			return nil, 0, err
		}
	}
	count, err := r.queries.CountProblemPackages(ctx, problemID)
	if err != nil {
		return nil, 0, dbError("list packages", err)
	}
	return result, safeInt32(count), nil
}

func (r *PackagesRepo) GetReadyPackage(ctx context.Context, problemID uuid.UUID) (problemsmodel.ProblemPackage, error) {
	row, err := r.queries.GetReadyPackage(ctx, problemID)
	if err != nil {
		return problemsmodel.ProblemPackage{}, dbError("get ready package", err)
	}
	return mapPackage(row)
}

func (r *PackagesRepo) GetPackageByID(ctx context.Context, id uuid.UUID) (problemsmodel.ProblemPackage, error) {
	row, err := r.queries.GetProblemPackageByID(ctx, id)
	if err != nil {
		return problemsmodel.ProblemPackage{}, dbError("get package by id", err)
	}
	return mapPackage(row)
}

func mapPackage(p sqlc.ProblemPackage) (problemsmodel.ProblemPackage, error) {
	result := problemsmodel.ProblemPackage{ID: p.ID, ProblemID: p.ProblemID, OrganizationID: p.OrganizationID, BuildID: p.BuildID, ObjectID: p.ObjectID, Revision: p.Revision,
		PackageHash: p.PackageHash, CreatedAt: p.CreatedAt, Title: p.Title, Type: p.Type, Grading: p.Grading,
		TimeLimitMs: p.TimeLimitMs, MemoryLimitMb: p.MemoryLimitMb, InputFile: p.InputFile, OutputFile: p.OutputFile, Files: p.FilesObjectID != nil}
	if err := json.Unmarshal(p.Statements, &result.Statements); err != nil {
		return result, dbError("decode package statements", err)
	}
	return result, nil
}
