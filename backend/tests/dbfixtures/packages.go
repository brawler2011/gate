package dbfixtures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A zero creation time uses the current time; publication bypasses the build pipeline.
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
	buildID, objectID, packageID := uuid.New(), uuid.New(), uuid.New()
	sum := sha256.Sum256(packageID[:])
	hash := hex.EncodeToString(sum[:])
	if _, err := tx.Exec(ctx, `INSERT INTO stored_objects(id,organization_id,purpose,key,sha256,size)
VALUES($1::uuid,$2::uuid,'package','objects/'||$2::text||'/'||$1::text,$3,1)`, objectID, organizationID, hash); err != nil {
		return uuid.Nil, fmt.Errorf("object: %w", err)
	}
	snapshot := fmt.Sprintf(`{"organization_id":%q,"input_objects":[],"draft":{"problem_id":%q,"revision":%d,"title":%q,"short_name":%q,"time_limit_ms":%d,"memory_limit_mb":%d,"tests":[],"assets":[]}}`,
		organizationID, problemID, revision, title, shortName, timeMs, memoryMb)
	if _, err := tx.Exec(ctx, `INSERT INTO problem_builds(id,problem_id,organization_id,revision,snapshot,state,lease_token,lease_deadline,attempt,max_attempts,deadline)
VALUES($1,$2,$3,$4,$5::jsonb,'running',$6,clock_timestamp()+interval '1 hour',1,1,clock_timestamp()+interval '1 hour')`,
		buildID, problemID, organizationID, revision, snapshot, uuid.New()); err != nil {
		return uuid.Nil, fmt.Errorf("build: %w", err)
	}
	if created.IsZero() {
		created = time.Now()
	}
	if _, err := tx.Exec(ctx, `INSERT INTO problem_packages(id,problem_id,organization_id,build_id,object_id,package_hash,created_at,revision,title,short_name,type,grading,time_limit_ms,memory_limit_mb,input_file,output_file,statements,samples)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'standard','pass-fail',$11,$12,'','','[]','[]')`,
		packageID, problemID, organizationID, buildID, objectID, hash, created, revision, title, shortName, timeMs, memoryMb); err != nil {
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

type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Zero Visibility means private.
type ProblemSpec struct {
	ID, OrganizationID, Owner uuid.UUID
	Visibility                string
	Title, ShortName          string
	CreatedAt                 time.Time
}

func InsertProblem(ctx context.Context, db Execer, spec ProblemSpec) error {
	if spec.Visibility == "" {
		spec.Visibility = "private"
	}
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = time.Now()
	}
	if _, err := db.Exec(ctx, `WITH p AS (INSERT INTO problems(id,organization_id,visibility,created_at) VALUES($1,$2,$3,$4) RETURNING id)
INSERT INTO problem_members(problem_id,user_id,role) SELECT id,$5,'owner' FROM p`,
		spec.ID, spec.OrganizationID, spec.Visibility, spec.CreatedAt, spec.Owner); err != nil {
		return fmt.Errorf("problem: %w", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO problem_drafts(problem_id,organization_id,title,short_name,type,grading,time_limit_ms,memory_limit_mb)
VALUES($1,$2,$3,$4,'standard','pass-fail',1000,256)`, spec.ID, spec.OrganizationID, spec.Title, spec.ShortName); err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	return nil
}

// Resources keep a permanent owner, so each insert adds the owner in the same statement.
func InsertOrganization(ctx context.Context, db Execer, id uuid.UUID, login, name string, owner uuid.UUID) error {
	if _, err := db.Exec(ctx, `WITH o AS (INSERT INTO organizations(id,login,name) VALUES($1,$2,$3) RETURNING id)
INSERT INTO organization_members(organization_id,user_id,role) SELECT id,$4,'owner' FROM o`, id, login, name, owner); err != nil {
		return fmt.Errorf("organization: %w", err)
	}
	return nil
}

// Zero Visibility means private.
type ContestSpec struct {
	ID, OrganizationID, Owner uuid.UUID
	Login, Title, Visibility  string
	Start, End                *time.Time
}

func InsertContest(ctx context.Context, db Execer, spec ContestSpec) error {
	if spec.Visibility == "" {
		spec.Visibility = "private"
	}
	if spec.Title == "" {
		spec.Title = spec.Login
	}
	if _, err := db.Exec(ctx, `WITH c AS (INSERT INTO contests(id,organization_id,login,title,visibility,start_time,end_time) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id)
INSERT INTO contest_members(contest_id,user_id,role) SELECT id,$8,'owner' FROM c`,
		spec.ID, spec.OrganizationID, spec.Login, spec.Title, spec.Visibility, spec.Start, spec.End, spec.Owner); err != nil {
		return fmt.Errorf("contest: %w", err)
	}
	return nil
}

// CreateContest stores p with owner as its first owner.
func CreateContest(ctx context.Context, db Execer, p contestsmodel.CreateContestParams, owner uuid.UUID) error {
	if err := InsertContest(ctx, db, ContestSpec{ID: p.ID, OrganizationID: p.OrganizationID, Owner: owner, Login: p.Login, Title: p.Title,
		Visibility: string(p.Visibility), Start: p.StartTime, End: p.EndTime}); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `UPDATE contests SET description=$2 WHERE id=$1`, p.ID, p.Description); err != nil {
		return fmt.Errorf("contest description: %w", err)
	}
	return nil
}

// InsertUser stores a plain user that can hold resource roles.
func InsertUser(ctx context.Context, db Execer, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO users(id,username,password_hash) VALUES($1,$2,'unused')`, id, "user_"+strings.ReplaceAll(id.String(), "-", "")[:20]); err != nil {
		return fmt.Errorf("user: %w", err)
	}
	return nil
}
