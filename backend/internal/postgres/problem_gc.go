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

// ListUnreferencedObjects lists objects that no row references right now.
func (r *ProblemDraftsRepo) ListUnreferencedObjects(ctx context.Context) ([]model.StoredObject, error) {
	rows, err := sqlc.New(r.pool).ListUnreferencedObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list unreferenced objects: %w", err)
	}
	objects := make([]model.StoredObject, len(rows))
	for i, row := range rows {
		objects[i] = model.StoredObject{ID: row.ID, Key: row.Key}
	}
	return objects, nil
}

// DeleteObject deletes an object row unless a reference holds it. The foreign
// keys decide, including for a reference committed concurrently.
func (r *ProblemDraftsRepo) DeleteObject(ctx context.Context, id uuid.UUID) (bool, error) {
	err := sqlc.New(r.pool).DeleteStoredObject(ctx, id)
	if isForeignKeyViolation(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete stored object: %w", err)
	}
	return true, nil
}

// ExistingObjectKeys returns which of the given keys have an object row.
func (r *ProblemDraftsRepo) ExistingObjectKeys(ctx context.Context, keys []string) (map[string]bool, error) {
	rows, err := sqlc.New(r.pool).ListStoredObjectKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("list stored object keys: %w", err)
	}
	existing := make(map[string]bool, len(rows))
	for _, key := range rows {
		existing[key] = true
	}
	return existing, nil
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

// prunePackage re-checks eligibility under the Build then Package locks, so a
// consumer created meanwhile keeps the Package alive.
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
	if _, err := q.LockProblemPackage(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("lock package: %w", err)
	}
	eligible, err := q.PackagePruneEligible(ctx, sqlc.PackagePruneEligibleParams{RetentionMs: retention.Milliseconds(), ID: id})
	if err != nil {
		return fmt.Errorf("check package retention: %w", err)
	}
	if !eligible {
		return nil
	}
	if err := q.DeleteProblemPackage(ctx, id); err != nil {
		return fmt.Errorf("delete package: %w", err)
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
	if err := q.DeleteProblemBuild(ctx, id); err != nil {
		return fmt.Errorf("delete build: %w", err)
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
