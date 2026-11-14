package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mapStoredObject(o sqlc.StoredObject) model.StoredObject {
	return model.StoredObject{ID: o.ID, OrganizationID: o.OrganizationID, Purpose: o.Purpose, Key: o.Key, SHA256: o.Sha256, Size: o.Size, State: o.State, UploadDeadline: o.UploadDeadline}
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

func (r *ProblemDraftsRepo) RegisterBuildOutput(ctx context.Context, id, token uuid.UUID, digest string, size int64, uploadTTL time.Duration) (model.StoredObject, error) {
	if uploadTTL <= 0 || size < 0 {
		return model.StoredObject{}, pkg.ErrBadInput
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return model.StoredObject{}, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	b, err := q.LockProblemBuild(ctx, id)
	if err != nil {
		return model.StoredObject{}, HandlePgErr(err)
	}
	active, err := q.BuildLeaseValid(ctx, sqlc.BuildLeaseValidParams{Token: &token, ID: id})
	if err != nil {
		return model.StoredObject{}, fmt.Errorf("check build lease: %w", err)
	}
	if !active {
		return model.StoredObject{}, pkg.ErrConflict
	}
	// Resolves registration with a lost commit response without a second UUID.
	existing, err := q.BuildOutputObjectID(ctx, sqlc.BuildOutputObjectIDParams{BuildID: id, AttemptToken: &token})
	if err == nil {
		if _, err := lockObjects(ctx, tx, []uuid.UUID{existing}); err != nil {
			return model.StoredObject{}, err
		}
		o, err := q.ReadStoredObject(ctx, existing)
		if err != nil {
			return model.StoredObject{}, fmt.Errorf("read registered output: %w", err)
		}
		if o.Sha256 != digest || o.Size != size {
			return model.StoredObject{}, pkg.ErrConflict
		}
		return mapStoredObject(o), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.StoredObject{}, fmt.Errorf("read output registration: %w", err)
	}
	oid := uuid.New()
	if err := q.InsertPackageObject(ctx, sqlc.InsertPackageObjectParams{
		ID: oid, OrganizationID: b.OrganizationID, Key: fmt.Sprintf("objects/%s/%s", b.OrganizationID, oid),
		Sha256: digest, Size: size, BuildDeadline: b.Deadline, TtlMs: uploadTTL.Milliseconds(),
	}); err != nil {
		return model.StoredObject{}, fmt.Errorf("register output object: %w", err)
	}
	if err := q.InsertBuildOutput(ctx, sqlc.InsertBuildOutputParams{BuildID: id, ObjectID: oid, AttemptToken: &token}); err != nil {
		return model.StoredObject{}, fmt.Errorf("hold output object: %w", err)
	}
	o, err := q.ReadStoredObject(ctx, oid)
	if err != nil {
		return model.StoredObject{}, fmt.Errorf("read output object: %w", err)
	}
	if err := commitTx(ctx, tx); err != nil {
		return model.StoredObject{}, err
	}
	return mapStoredObject(o), nil
}

// PublishBuild is called only after the trusted worker confirmed upload bytes.
// The single transaction hands ownership from the attempt to Package.
// Statement samples and asset references are stored with the Package so that
// statement reads never open its archive.
func (r *ProblemDraftsRepo) PublishBuild(ctx context.Context, id, token, oid uuid.UUID, samples []model.Sample) (uuid.UUID, error) {
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
	var snapshot model.BuildSnapshot
	if err := json.Unmarshal(b.Snapshot, &snapshot); err != nil {
		return uuid.Nil, fmt.Errorf("decode build snapshot: %w", err)
	}
	ids := []uuid.UUID{oid}
	for _, a := range snapshot.Draft.Assets {
		ids = append(ids, a.ObjectID)
	}
	objects, err := lockObjects(ctx, tx, ids)
	if err != nil {
		return uuid.Nil, err
	}
	o := objects[oid]
	if o.OrganizationID != b.OrganizationID || o.Purpose != "package" || o.State != "uploading" {
		return uuid.Nil, pkg.ErrConflict
	}
	held, err := q.OutputUploadActive(ctx, sqlc.OutputUploadActiveParams{BuildID: id, ObjectID: oid, Token: &token})
	if err != nil {
		return uuid.Nil, fmt.Errorf("check output hold: %w", err)
	}
	if !held {
		return uuid.Nil, pkg.ErrConflict
	}
	if err := q.PublishStoredObject(ctx, oid); err != nil {
		return uuid.Nil, fmt.Errorf("publish package object: %w", err)
	}
	pid := uuid.New()
	if err := q.InsertProblemPackage(ctx, sqlc.InsertProblemPackageParams{
		ID: pid, ProblemID: b.ProblemID, OrganizationID: b.OrganizationID, BuildID: id, ObjectID: oid, PackageHash: o.Sha256, Revision: b.Revision, Samples: samplesJSON,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("insert package: %w", err)
	}
	if err := q.InsertPackageAssets(ctx, sqlc.InsertPackageAssetsParams{PackageID: pid, BuildID: id}); err != nil {
		return uuid.Nil, fmt.Errorf("insert package assets: %w", err)
	}
	if err := q.DeleteBuildOutput(ctx, sqlc.DeleteBuildOutputParams{BuildID: id, ObjectID: oid, AttemptToken: &token}); err != nil {
		return uuid.Nil, fmt.Errorf("release output hold: %w", err)
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

// BeginObjectDeletion intentionally uses two statements: a lock waiter must
// check references using a fresh READ COMMITTED snapshot after the lock arrives.
func (r *ProblemDraftsRepo) BeginObjectDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) (bool, error) {
	if retention <= 0 {
		return false, pkg.ErrBadInput
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return false, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	o, err := q.LockStoredObject(ctx, id)
	if err != nil {
		return false, HandlePgErr(err)
	}
	if o.State == "deleting" {
		return true, nil
	}
	if o.State == "deleted" {
		return false, nil
	}
	eligible, err := q.ObjectDeletionEligible(ctx, sqlc.ObjectDeletionEligibleParams{RetentionMs: retention.Milliseconds(), ID: id})
	if err != nil {
		return false, fmt.Errorf("check object references: %w", err)
	}
	if !eligible {
		return false, nil
	}
	if err := q.MarkObjectDeleting(ctx, id); err != nil {
		return false, fmt.Errorf("mark object deleting: %w", err)
	}
	return true, commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) CompleteObjectDeletion(ctx context.Context, id uuid.UUID) error {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := q.LockStoredObject(ctx, id); err != nil {
		return HandlePgErr(err)
	}
	updated, err := q.MarkObjectDeleted(ctx, id)
	if err != nil {
		return fmt.Errorf("mark object deleted: %w", err)
	}
	if updated != 1 {
		return pkg.ErrConflict
	}
	return commitTx(ctx, tx)
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
	p := model.PackageStatement{ProblemID: row.ProblemID}
	var snapshot model.BuildSnapshot
	if err := json.Unmarshal(row.Snapshot, &snapshot); err != nil {
		return p, fmt.Errorf("decode package snapshot: %w", err)
	}
	p.Draft = snapshot.Draft
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
		assets = append(assets, model.PackageAsset{Path: row.Path, Key: row.Key})
	}
	return assets, nil
}
