// Package dbfixtures inserts valid problem-package graphs for integration tests
// that need a pinned Package but do not exercise the build pipeline.
package dbfixtures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InsertPackage publishes a minimal successful Build and its Package for the
// problem in one transaction, honouring the database publication invariants.
// It creates the problem's draft when it does not exist yet. created is the
// package creation time; a zero value means now.
func InsertPackage(ctx context.Context, pool *pgxpool.Pool, problemID, organizationID uuid.UUID, created time.Time) (uuid.UUID, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return uuid.Nil, fmt.Errorf("begintx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var revision int64
	var title, shortName string
	var timeMs, memoryMb int
	if err := tx.QueryRow(ctx, `SELECT revision,title,short_name,time_limit_ms,memory_limit_mb FROM problem_drafts WHERE problem_id=$1`, problemID).
		Scan(&revision, &title, &shortName, &timeMs, &memoryMb); err != nil {
		return uuid.Nil, fmt.Errorf("read draft: %w", err)
	}
	// Builds are unique per problem revision only by id, so every package gets its own build.
	buildID, objectID, packageID := uuid.New(), uuid.New(), uuid.New()
	sum := sha256.Sum256(packageID[:])
	hash := hex.EncodeToString(sum[:])
	toolchain := "sha256:" + hex.EncodeToString(sum[:])
	if _, err := tx.Exec(ctx, `INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size,state,upload_deadline)
VALUES($1::uuid,$2::uuid,'package','objects/'||$2::text||'/'||$1::text,$3,1,'uploading',clock_timestamp()+interval '1 hour')`, objectID, organizationID, hash); err != nil {
		return uuid.Nil, fmt.Errorf("object: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE stored_objects SET state='available' WHERE id=$1`, objectID); err != nil {
		return uuid.Nil, fmt.Errorf("publish object: %w", err)
	}
	snapshot := fmt.Sprintf(`{"schema_version":1,"organization_id":%q,"toolchain":{"digest":%q},"input_objects":[],"draft":{"problem_id":%q,"revision":%d,"title":%q,"short_name":%q,"time_limit_ms":%d,"memory_limit_mb":%d,"tests":[],"assets":[]}}`,
		organizationID, toolchain, problemID, revision, title, shortName, timeMs, memoryMb)
	if _, err := tx.Exec(ctx, `INSERT INTO problem_builds(id,problem_id,organization_id,revision,snapshot,schema_version,toolchain_digest,state,lease_token,lease_deadline,attempt,max_attempts,deadline)
VALUES($1,$2,$3,$4,$5::jsonb,1,$6,'running',$7,clock_timestamp()+interval '1 hour',1,1,clock_timestamp()+interval '1 hour')`,
		buildID, problemID, organizationID, revision, snapshot, toolchain, uuid.New()); err != nil {
		return uuid.Nil, fmt.Errorf("build: %w", err)
	}
	if created.IsZero() {
		created = time.Now()
	}
	if _, err := tx.Exec(ctx, `INSERT INTO problem_packages(id,problem_id,organization_id,build_id,object_id,package_hash,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		packageID, problemID, organizationID, buildID, objectID, hash, created); err != nil {
		return uuid.Nil, fmt.Errorf("package: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE problem_builds SET state='succeeded',lease_token=NULL,lease_deadline=NULL,finished_at=clock_timestamp() WHERE id=$1`, buildID); err != nil {
		return uuid.Nil, fmt.Errorf("finish build: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return packageID, nil
}

// Execer is satisfied by pgx pools and transactions.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// ProblemSpec describes a problem row and its draft. Zero Visibility means private.
type ProblemSpec struct {
	ID, OrganizationID uuid.UUID
	OwnerID            *uuid.UUID
	Visibility         string
	Title, ShortName   string
	CreatedAt          time.Time
}

// InsertProblem creates the problem identity row and its pass-fail draft, like problem creation does.
func InsertProblem(ctx context.Context, db Execer, spec ProblemSpec) error {
	if spec.Visibility == "" {
		spec.Visibility = "private"
	}
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = time.Now()
	}
	if _, err := db.Exec(ctx, `INSERT INTO problems(id,organization_id,owner_id,visibility,created_at) VALUES($1,$2,$3,$4,$5)`,
		spec.ID, spec.OrganizationID, spec.OwnerID, spec.Visibility, spec.CreatedAt); err != nil {
		return fmt.Errorf("problem: %w", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO problem_drafts(problem_id,organization_id,title,short_name,type,time_limit_ms,memory_limit_mb)
VALUES($1,$2,$3,$4,'pass-fail',1000,256)`, spec.ID, spec.OrganizationID, spec.Title, spec.ShortName); err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	return nil
}
