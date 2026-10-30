//go:build integration

package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func workspaceSessionDatabase(t *testing.T) (*pgx.ConnConfig, *pgx.Conn, *ProblemsRepo, *PackagesRepo, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	config := workspaceLockTestConfig(t)
	admin, err := pgx.ConnectConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, admin.Close(context.Background())) })
	schema := "workspace_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
	})
	config.RuntimeParams["search_path"] = schema
	config.RuntimeParams["application_name"] = schema
	require.NoError(t, admin.Close(ctx))
	admin, err = pgx.ConnectConfig(ctx, config)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `CREATE TYPE package_status AS ENUM ('building','ready');
 CREATE TABLE problems (id uuid PRIMARY KEY,manifest jsonb,title text,time_limit_ms integer,memory_limit_mb integer,visibility text,owner_id uuid,is_template boolean);
 CREATE TABLE problem_packages (id uuid PRIMARY KEY,problem_id uuid,organization_id uuid,version integer,package_hash text,url text,status package_status,build_log text,created_at timestamptz DEFAULT now(),compiled_at timestamptz);`)
	require.NoError(t, err)
	poolConfig, err := pgxpool.ParseConfig(config.ConnString())
	require.NoError(t, err)
	poolConfig.ConnConfig = config.Copy()
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "workspace-repository-pool"
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	id := uuid.New()
	_, err = admin.Exec(ctx, "INSERT INTO problems(id,title,manifest,time_limit_ms,memory_limit_mb) VALUES($1,'Workspace',NULL,1000,256)", id)
	require.NoError(t, err)
	return config, admin, NewProblemsRepo(pool), NewPackagesRepo(pool), id
}

func terminateWorkspaceSession(t *testing.T, admin *pgx.Conn, ctx context.Context) {
	t.Helper()
	session := ctx.Value(workspaceSessionKey{}).(*workspaceSession)
	var terminated bool
	require.NoError(t, admin.QueryRow(context.Background(), "SELECT pg_terminate_backend($1,5000)", session.conn.PgConn().PID()).Scan(&terminated))
	require.True(t, terminated)
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("lost session did not cancel operation")
	}
}

func TestWorkspaceLocker_LostSessionRejectsLateDatabaseWrites(t *testing.T) {
	config, admin, problems, packages, id := workspaceSessionDatabase(t)
	ctx := context.Background()
	var packageID uuid.UUID
	var callbackContext context.Context
	err := NewWorkspaceLocker(config).WithWorkspaceLock(ctx, id, func(operation context.Context) error {
		callbackContext = operation
		require.NoError(t, problems.UpdateProblemManifest(operation, id, []byte(`{"original":true}`)))
		created, err := packages.CreatePackage(operation, &models.CreatePackageParams{ID: uuid.New(), ProblemID: id, OrganizationID: uuid.New(), PackageHash: "old", Status: "building"})
		require.NoError(t, err)
		packageID = created.ID
		terminateWorkspaceSession(t, admin, operation)
		// A successor can acquire the server-side lock while arbitrary old Go code
		// still runs. Its authoritative commit must survive the old owner's attempts.
		require.NoError(t, NewWorkspaceLocker(config).WithWorkspaceLock(ctx, id, func(next context.Context) error {
			return problems.UpdateProblemManifest(next, id, []byte(`{"successor":true}`))
		}))
		// Even replacing the canceled parent with WithoutCancel cannot revive the
		// dead owning connection or redirect critical writes through the live pool.
		detached := context.WithoutCancel(operation)
		require.Error(t, problems.UpdateProblemManifest(detached, id, []byte(`{"stale":true}`)))
		require.Error(t, problems.UpdateProblemLimits(detached, id, 9999, 999))
		title := "stale"
		require.Error(t, problems.UpdateProblem(detached, id, &models.ProblemUpdate{Title: &title}))
		_, err = packages.CreatePackage(detached, &models.CreatePackageParams{ID: uuid.New(), ProblemID: id, OrganizationID: uuid.New(), PackageHash: "stale", Status: "ready"})
		require.Error(t, err)
		require.Error(t, packages.UpdatePackageStatus(detached, &models.UpdatePackageStatusParams{ID: packageID, Status: "ready"}))
		return nil
	})
	require.ErrorContains(t, err, "session lost")
	require.Error(t, callbackContext.Err())
	var manifest []byte
	var timeLimit int
	require.NoError(t, admin.QueryRow(ctx, "SELECT manifest,time_limit_ms FROM problems WHERE id=$1", id).Scan(&manifest, &timeLimit))
	require.JSONEq(t, `{"successor":true}`, string(manifest))
	require.Equal(t, 1000, timeLimit)
	var status string
	require.NoError(t, admin.QueryRow(ctx, "SELECT status FROM problem_packages WHERE id=$1", packageID).Scan(&status))
	require.Equal(t, "building", status)
	// Ordinary repository calls outside the ownership context still use the pool.
	require.NoError(t, problems.UpdateProblemLimits(ctx, id, 2000, 256))
	require.NoError(t, NewWorkspaceLocker(config).WithWorkspaceLock(ctx, id, func(context.Context) error { return nil }))
}

type workspaceFixtureRepo struct {
	interfaces.ProblemsRepo
	problem models.Problem
}

func (r *workspaceFixtureRepo) GetProblemById(context.Context, uuid.UUID) (models.Problem, error) {
	return r.problem, nil
}

type terminateAfterUploadStorage struct {
	storage.Storage
	suffix string
	after  func(context.Context)
}

func (s *terminateAfterUploadStorage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	if err := s.Storage.UploadFile(ctx, bucket, key, reader, contentType); err != nil {
		return fmt.Errorf("write production local storage: %w", err)
	}
	if strings.HasSuffix(key, s.suffix) {
		s.after(ctx)
	}
	return nil
}

func TestWorkspaceLocker_LossDuringMutationAndPublication(t *testing.T) {
	for _, operation := range []string{"mutation", "publication"} {
		t.Run(operation, func(t *testing.T) {
			config, admin, problems, packages, id := workspaceSessionDatabase(t)
			ctx := context.Background()
			repo := &workspaceFixtureRepo{ProblemsRepo: problems, problem: models.Problem{ID: id, Title: "Workspace", OrganizationID: uuid.New()}}
			local := storage.NewLocalStorage(t.TempDir())
			ws := usecase.NewWorkspaceStorage(local, "workspace", NewWorkspaceLocker(config))
			uc := usecase.NewWorkshopUseCase(repo, ws, nil, nil)
			require.NoError(t, uc.InitProblemWorkshop(ctx, id, "Workspace"))
			require.NoError(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "checkers/check.cpp", Content: []byte("old checker")}))
			before, err := problems.GetProblemManifest(ctx, id)
			require.NoError(t, err)
			packageStorage := storage.Storage(local)
			if operation == "mutation" {
				wrapped := &terminateAfterUploadStorage{Storage: local, suffix: "checkers/check.cpp", after: func(ctx context.Context) { terminateWorkspaceSession(t, admin, ctx) }}
				ws = usecase.NewWorkspaceStorage(wrapped, "workspace", NewWorkspaceLocker(config))
				uc = usecase.NewWorkshopUseCase(repo, ws, nil, nil)
				err = uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "checkers/check.cpp", Content: []byte("partial checker")})
				require.ErrorContains(t, err, "session lost")
				after, err := problems.GetProblemManifest(ctx, id)
				require.NoError(t, err)
				require.Equal(t, json.RawMessage(before), json.RawMessage(after))
				_, err = uc.GetManifest(ctx, id)
				require.ErrorContains(t, err, "incomplete")
				publisher := usecase.NewProblemPublishUseCase(repo, packages, ws, local, "published")
				_, err = publisher.PublishProblem(ctx, id)
				require.ErrorContains(t, err, "incomplete")
				// Retry with the production store after the failed operation has completed.
				ws = usecase.NewWorkspaceStorage(local, "workspace", NewWorkspaceLocker(config))
				uc = usecase.NewWorkshopUseCase(repo, ws, nil, nil)
				require.NoError(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "checkers/check.cpp", Content: []byte("complete checker")}))
				_, err = uc.GetManifest(ctx, id)
				require.NoError(t, err)
			} else {
				packageStorage = &terminateAfterUploadStorage{Storage: local, suffix: ".zip", after: func(ctx context.Context) { terminateWorkspaceSession(t, admin, ctx) }}
				publisher := usecase.NewProblemPublishUseCase(repo, packages, ws, packageStorage, "published")
				_, err = publisher.PublishProblem(ctx, id)
				require.ErrorContains(t, err, "session lost")
			}
			var count int
			require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM problem_packages WHERE problem_id=$1", id).Scan(&count))
			require.Zero(t, count)
		})
	}
}
