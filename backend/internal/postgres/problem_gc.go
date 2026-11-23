package postgres

import (
	"context"
	"errors"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DeleteUnreferencedObjects returns the storage keys of the deleted rows.
func (r *ProblemDraftsRepo) DeleteUnreferencedObjects(ctx context.Context) ([]string, error) {
	keys, err := sqlc.New(r.pool).DeleteUnreferencedObjects(ctx)
	if err != nil {
		return nil, dbError("delete unreferenced objects", err)
	}
	return keys, nil
}

func (r *ProblemDraftsRepo) ExistingObjectKeys(ctx context.Context, keys []string) (map[string]bool, error) {
	rows, err := sqlc.New(r.pool).ListStoredObjectKeys(ctx, keys)
	if err != nil {
		return nil, dbError("list stored object keys", err)
	}
	existing := make(map[string]bool, len(rows))
	for _, key := range rows {
		existing[key] = true
	}
	return existing, nil
}

func (r *ProblemDraftsRepo) PrunePackages(ctx context.Context, retention time.Duration) error {
	if retention <= 0 {
		return pkg.ErrInvalid
	}
	ids, err := sqlc.New(r.pool).ListPackageIDsPastRetention(ctx, retention.Milliseconds())
	if err != nil {
		return dbError("list packages past retention", err)
	}
	for _, id := range ids {
		if err := r.prunePackage(ctx, id, retention); err != nil {
			return err
		}
	}
	return nil
}

// Lock Build then Package and recheck eligibility so concurrent consumers keep the package alive.
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
		return dbError("read package build", err)
	}
	if _, err := q.LockProblemBuild(ctx, buildID); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return dbError("lock build", err)
	}
	if _, err := q.LockProblemPackage(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return dbError("lock package", err)
	}
	eligible, err := q.PackagePruneEligible(ctx, sqlc.PackagePruneEligibleParams{RetentionMs: retention.Milliseconds(), ID: id})
	if err != nil {
		return dbError("check package retention", err)
	}
	if !eligible {
		return nil
	}
	if err := q.DeleteProblemPackage(ctx, id); err != nil {
		return dbError("delete package", err)
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) PruneBuilds(ctx context.Context, retention time.Duration) error {
	if retention <= 0 {
		return pkg.ErrInvalid
	}
	ids, err := sqlc.New(r.pool).ListBuildIDsPastRetention(ctx, retention.Milliseconds())
	if err != nil {
		return dbError("list builds past retention", err)
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
		return dbError("lock build", err)
	}
	eligible, err := q.BuildPruneEligible(ctx, sqlc.BuildPruneEligibleParams{RetentionMs: retention.Milliseconds(), ID: id})
	if err != nil {
		return dbError("check build retention", err)
	}
	if !eligible {
		return nil
	}
	if err := q.DeleteProblemBuild(ctx, id); err != nil {
		return dbError("delete build", err)
	}
	return commitTx(ctx, tx)
}

func (r *ProblemDraftsRepo) ActiveBuildAttempt(ctx context.Context, id, token uuid.UUID) (bool, error) {
	active, err := sqlc.New(r.pool).BuildAttemptActive(ctx, sqlc.BuildAttemptActiveParams{ID: id, LeaseToken: &token})
	if err != nil {
		return false, dbError("check build attempt", err)
	}
	return active, nil
}
