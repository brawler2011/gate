package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *ProblemDraftsRepo) ListGCObjects(ctx context.Context, retention time.Duration) ([]model.StoredObject, error) {
	q := sqlc.New(r.pool)
	ids, err := q.ListGCObjectIDs(ctx, retention.Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("list objects for collection: %w", err)
	}
	result := make([]model.StoredObject, 0, len(ids))
	for _, id := range ids {
		o, err := q.ReadStoredObject(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("read object %s: %w", id, err)
		}
		result = append(result, mapStoredObject(o))
	}
	return result, nil
}

func (r *ProblemDraftsRepo) ObjectByKey(ctx context.Context, key string) (model.StoredObject, error) {
	q := sqlc.New(r.pool)
	id, err := q.StoredObjectIDByKey(ctx, key)
	if err != nil {
		return model.StoredObject{}, HandlePgErr(err)
	}
	o, err := q.ReadStoredObject(ctx, id)
	if err != nil {
		return model.StoredObject{}, HandlePgErr(err)
	}
	return mapStoredObject(o), nil
}

func (r *ProblemDraftsRepo) PrunePackages(ctx context.Context, retention time.Duration) error {
	if retention <= 0 {
		return pkg.ErrBadInput
	}
	ids, err := sqlc.New(r.pool).ListPackageIDsPastRetention(ctx, retention.Milliseconds())
	if err != nil {
		return fmt.Errorf("list packages past retention: %w", err)
	}
	for _, id := range ids {
		if err := r.prunePackage(ctx, id, retention); err != nil {
			return err
		}
	}
	return nil
}

// prunePackage re-checks eligibility under the Build then Package then object
// locks, so a consumer created meanwhile keeps the Package alive.
func (r *ProblemDraftsRepo) prunePackage(ctx context.Context, id uuid.UUID, retention time.Duration) error {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	// Read the candidate identity before locking, then recheck under the locks.
	buildID, err := q.PackageBuildID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read package build: %w", err)
	}
	if _, err := q.LockProblemBuild(ctx, buildID); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("lock build: %w", err)
	}
	objectID, err := q.LockPackageObject(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock package: %w", err)
	}
	eligible, err := q.PackagePruneEligible(ctx, sqlc.PackagePruneEligibleParams{RetentionMs: retention.Milliseconds(), ID: id})
	if err != nil {
		return fmt.Errorf("check package retention: %w", err)
	}
	if !eligible {
		return nil
	}
	assetIDs, err := q.ListPackageAssetObjectIDs(ctx, id)
	if err != nil {
		return fmt.Errorf("list package assets: %w", err)
	}
	objectIDs := append([]uuid.UUID{objectID}, assetIDs...)
	// Deleting the Package bumps the access revision; take it before the
	// asset objects, which a template clone may lock after it.
	if err := q.LockAccessRevision(ctx); err != nil {
		return fmt.Errorf("lock access revision: %w", err)
	}
	if _, err := lockObjects(ctx, tx, objectIDs); err != nil {
		return err
	}
	if err := q.DeleteProblemPackage(ctx, id); err != nil {
		return fmt.Errorf("delete package: %w", err)
	}
	if err := refreshObjectReferences(ctx, tx, objectIDs); err != nil {
		return err
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) PruneBuilds(ctx context.Context, retention time.Duration) error {
	if retention <= 0 {
		return pkg.ErrBadInput
	}
	ids, err := sqlc.New(r.pool).ListBuildIDsPastRetention(ctx, retention.Milliseconds())
	if err != nil {
		return fmt.Errorf("list builds past retention: %w", err)
	}
	for _, id := range ids {
		if err := r.pruneBuild(ctx, id, retention); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProblemDraftsRepo) pruneBuild(ctx context.Context, id uuid.UUID, retention time.Duration) error {
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	if _, err := q.LockProblemBuild(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("lock build: %w", err)
	}
	eligible, err := q.BuildPruneEligible(ctx, sqlc.BuildPruneEligibleParams{RetentionMs: retention.Milliseconds(), ID: id})
	if err != nil {
		return fmt.Errorf("check build retention: %w", err)
	}
	if !eligible {
		return nil
	}
	objectIDs, err := q.ListBuildObjectIDs(ctx, id)
	if err != nil {
		return fmt.Errorf("list build objects: %w", err)
	}
	if _, err := lockObjects(ctx, tx, objectIDs); err != nil {
		return err
	}
	if err := q.DeleteProblemBuild(ctx, id); err != nil {
		return fmt.Errorf("delete build: %w", err)
	}
	if err := refreshObjectReferences(ctx, tx, objectIDs); err != nil {
		return err
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) ActiveBuildAttempt(ctx context.Context, id, token uuid.UUID) (bool, error) {
	active, err := sqlc.New(r.pool).BuildAttemptActive(ctx, sqlc.BuildAttemptActiveParams{ID: id, LeaseToken: &token})
	if err != nil {
		return false, fmt.Errorf("check build attempt: %w", err)
	}
	return active, nil
}
