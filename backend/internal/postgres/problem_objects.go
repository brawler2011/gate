package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func mapStoredObject(o sqlc.StoredObject) model.StoredObject {
	return model.StoredObject{ID: o.ID, OrganizationID: o.OrganizationID, Purpose: o.Purpose, Key: o.Key, SHA256: o.Sha256, Size: o.Size}
}
func (r *ProblemDraftsRepo) BuildInputObjects(ctx context.Context, id uuid.UUID) ([]model.StoredObject, error) {
	rows, err := sqlc.New(r.pool).ListBuildInputObjects(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list build inputs: %w", err)
	}
	objects := make([]model.StoredObject, 0, len(rows))
	for _, o := range rows {
		objects = append(objects, mapStoredObject(o))
	}
	return objects, nil
}

// PublishBuild is called after the worker stored the archive under the
// object's key with a verified SHA-256. One transaction records the object
// with the Package that references it, so an attempt that loses its lease
// leaves only bytes without a row. Statement samples and asset references are
// stored with the Package so that statement reads never open its archive.
func (r *ProblemDraftsRepo) PublishBuild(ctx context.Context, id, token uuid.UUID, object model.StoredObject, samples []model.Sample) (uuid.UUID, error) {
	if samples == nil {
		samples = []model.Sample{}
	}
	samplesJSON, err := json.Marshal(samples)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal samples: %w", err)
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return uuid.Nil, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	b, err := q.LockProblemBuild(ctx, id)
	if err != nil {
		return uuid.Nil, HandlePgErr(err)
	}
	if b.State == "succeeded" {
		pid, err := q.BuildPackageID(ctx, id)
		if err != nil {
			return uuid.Nil, HandlePgErr(err)
		}
		return pid, nil
	}
	active, err := q.BuildLeaseValid(ctx, sqlc.BuildLeaseValidParams{Token: &token, ID: id})
	if err != nil {
		return uuid.Nil, fmt.Errorf("check build lease: %w", err)
	}
	if !active {
		return uuid.Nil, pkg.ErrConflict
	}
	if object.OrganizationID != b.OrganizationID || object.Purpose != "package" {
		return uuid.Nil, pkg.ErrBadInput
	}
	if err := insertObjects(ctx, tx, []model.StoredObject{object}); err != nil {
		return uuid.Nil, err
	}
	pid := uuid.New()
	if err := q.InsertProblemPackage(ctx, sqlc.InsertProblemPackageParams{
		ID: pid, ProblemID: b.ProblemID, OrganizationID: b.OrganizationID, BuildID: id, ObjectID: object.ID, PackageHash: object.SHA256, Revision: b.Revision, Samples: samplesJSON,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("insert package: %w", err)
	}
	if err := q.InsertPackageAssets(ctx, sqlc.InsertPackageAssetsParams{PackageID: pid, BuildID: id}); err != nil {
		return uuid.Nil, fmt.Errorf("insert package assets: %w", err)
	}
	if err := q.MarkBuildSucceeded(ctx, sqlc.MarkBuildSucceededParams{ID: id, LeaseToken: &token}); err != nil {
		return uuid.Nil, fmt.Errorf("finish build: %w", err)
	}
	if err := commitTx(ctx, tx); err != nil {
		resolveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		durable, readErr := sqlc.New(r.pool).SucceededBuildPackageID(resolveCtx, id)
		if readErr == nil {
			return durable, nil
		}
		return uuid.Nil, err
	}
	return pid, nil
}

func (r *ProblemDraftsRepo) GetObject(ctx context.Context, id uuid.UUID) (model.StoredObject, error) {
	o, err := sqlc.New(r.pool).ReadStoredObject(ctx, id)
	if err != nil {
		return model.StoredObject{}, HandlePgErr(err)
	}
	return mapStoredObject(o), nil
}

func (r *ProblemDraftsRepo) GetPackageDescriptor(ctx context.Context, id uuid.UUID) (model.PackageDescriptor, error) {
	row, err := sqlc.New(r.pool).ReadPackageDescriptor(ctx, id)
	if err != nil {
		return model.PackageDescriptor{}, HandlePgErr(err)
	}
	p := model.PackageDescriptor{ID: row.ID, BuildID: row.BuildID, ProblemID: row.ProblemID}
	if err := json.Unmarshal(row.Snapshot, &p.Snapshot); err != nil {
		return p, fmt.Errorf("decode package snapshot: %w", err)
	}
	p.Object, err = r.GetObject(ctx, row.ObjectID)
	return p, err
}

func (r *ProblemDraftsRepo) GetPackageStatement(ctx context.Context, id uuid.UUID) (model.PackageStatement, error) {
	row, err := sqlc.New(r.pool).ReadPackageStatement(ctx, id)
	if err != nil {
		return model.PackageStatement{}, HandlePgErr(err)
	}
	p := model.PackageStatement{ProblemID: row.ProblemID, Title: row.Title}
	if err := json.Unmarshal(row.Statements, &p.Statements); err != nil {
		return p, fmt.Errorf("decode package statements: %w", err)
	}
	if err := json.Unmarshal(row.Samples, &p.Samples); err != nil {
		return p, fmt.Errorf("decode package samples: %w", err)
	}
	if err := json.Unmarshal(row.Assets, &p.Assets); err != nil {
		return p, fmt.Errorf("decode package assets: %w", err)
	}
	return p, nil
}

// FindPackageAssets lists the statement assets of a Package with the given
// content hash.
func (r *ProblemDraftsRepo) FindPackageAssets(ctx context.Context, id uuid.UUID, sha256 string) ([]model.PackageAsset, error) {
	rows, err := sqlc.New(r.pool).ListPackageAssetsBySHA256(ctx, sqlc.ListPackageAssetsBySHA256Params{PackageID: id, Sha256: sha256})
	if err != nil {
		return nil, fmt.Errorf("list package assets: %w", err)
	}
	assets := make([]model.PackageAsset, 0, len(rows))
	for _, row := range rows {
		assets = append(assets, model.PackageAsset{Name: row.Name, Key: row.Key})
	}
	return assets, nil
}
