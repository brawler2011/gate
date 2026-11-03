//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/brawler2011/gate/backend/internal/templates"

	"github.com/brawler2011/gate/backend/pkg"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/storage"
)

func (s *IntegrationTestSuite) TestProblemPatchRollsBackWorkspaceAndCatalogTogether() {
	owner := s.createUser("problem_patch_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("problem-patch", "Patch", owner.Id)
	workspace := problems.NewWorkspaceStorage(storage.NewLocalStorage(s.T().TempDir()), "workspaces", s.problemsRepo)
	uc := problems.NewProblemsUseCase(s.problemsRepo, workspace, pgstore.NewProblemTransactions(s.dbPool), nil, s.organizationsRepo)
	ctx := accessmodel.WithUser(s.ctx, owner)
	id, err := uc.CreateProblem(ctx, &problemsmodel.CreateProblemInput{OrganizationID: org.ID, Title: "Original", TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	before, err := s.problemsRepo.GetWorkspaceHash(s.ctx, id)
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_problem_patch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected catalog failure'; END $$;
 CREATE TRIGGER reject_problem_patch BEFORE UPDATE OF visibility ON problems FOR EACH ROW EXECUTE FUNCTION reject_problem_patch()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER IF EXISTS reject_problem_patch ON problems; DROP FUNCTION reject_problem_patch()`)
		s.Require().NoError(err)
	}()
	title := "Changed"
	s.Require().Error(uc.UpdateProblem(ctx, id, &problemsmodel.ProblemUpdate{Title: &title}))
	hash, err := s.problemsRepo.GetWorkspaceHash(s.ctx, id)
	s.Require().NoError(err)
	s.Equal(before, hash, "the prepared immutable ZIP must not become visible after rollback")
	catalog, err := s.problemsRepo.GetProblemById(s.ctx, id)
	s.Require().NoError(err)
	s.Equal("Original", catalog.Title)
	workshop := problems.NewWorkshopUseCase(workspace, nil)
	manifest, err := workshop.GetManifest(s.ctx, id)
	s.Require().NoError(err)
	s.Equal("Original", manifest.Statement.Title)
	_, err = s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_problem_patch ON problems`)
	s.Require().NoError(err)
	s.Require().NoError(uc.UpdateProblem(ctx, id, &problemsmodel.ProblemUpdate{Title: &title}))
	manifest, err = workshop.GetManifest(s.ctx, id)
	s.Require().NoError(err)
	s.Equal(title, manifest.Statement.Title)
	catalog, err = s.problemsRepo.GetProblemById(s.ctx, id)
	s.Require().NoError(err)
	s.Equal(title, catalog.Title)
}

func (s *IntegrationTestSuite) TestConcurrentIdenticalPublicationRetainsCompleteVersion() {
	owner := s.createUser("publish_atomic_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("publish-atomic", "Publish", owner.Id)
	objects := storage.NewLocalStorage(s.T().TempDir())
	workspace := problems.NewWorkspaceStorage(objects, "workspaces", s.problemsRepo)
	transactions := pgstore.NewProblemTransactions(s.dbPool)
	uc := problems.NewProblemsUseCase(s.problemsRepo, workspace, transactions, nil, s.organizationsRepo)
	ctx := accessmodel.WithUser(s.ctx, owner)
	id, err := uc.CreateProblem(ctx, &problemsmodel.CreateProblemInput{OrganizationID: org.ID, Title: "Publish", TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	publisher := problems.NewProblemPublishUseCase(s.problemsRepo, pgstore.NewPackagesRepo(s.dbPool), workspace, objects, "packages", transactions)
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, err := publisher.PublishProblem(ctx, id); failures <- err })
	}
	wg.Wait()
	close(failures)
	conflicts := 0
	for err := range failures {
		if errors.Is(err, pkg.ErrConflict) {
			conflicts++
		} else {
			s.Require().NoError(err)
		}
	}
	s.Equal(1, conflicts, "publishing identical content preserves the unique package hash rule")
	packages, err := publisher.ListPackages(ctx, id)
	s.Require().NoError(err)
	s.Require().Len(packages, 1)
	versions := map[int32]bool{}
	for _, p := range packages {
		versions[p.Version] = true
		s.Equal("ready", p.Status)
		s.NotNil(p.CompiledAt)
	}
	s.True(versions[1])
	s.False(versions[2])
}

type problemUploadHook struct {
	storage.Storage
	before func(context.Context) error
}

func (s *problemUploadHook) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	if s.before != nil {
		if err := s.before(ctx); err != nil {
			return err
		}
	}
	return s.Storage.UploadFile(ctx, bucket, key, reader, contentType)
}

func (s *IntegrationTestSuite) TestImportRechecksAccessAfterObjectUpload() {
	owner := s.createUser("import_revoke_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("import-revoke", "Import", owner.Id)
	objects := &problemUploadHook{Storage: storage.NewLocalStorage(s.T().TempDir())}
	workspace := problems.NewWorkspaceStorage(objects, "workspaces", s.problemsRepo)
	transactions := pgstore.NewProblemTransactions(s.dbPool)
	uc := problems.NewProblemsUseCase(s.problemsRepo, workspace, transactions, nil, s.organizationsRepo)
	ctx := accessmodel.WithUser(s.ctx, owner)
	id, err := uc.CreateProblem(ctx, &problemsmodel.CreateProblemInput{OrganizationID: org.ID, Title: "Original", TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	previous, err := s.problemsRepo.GetWorkspaceHash(s.ctx, id)
	s.Require().NoError(err)
	objects.before = func(ctx context.Context) error {
		_, err := s.dbPool.Exec(ctx, `WITH membership AS (UPDATE organization_members SET role='member' WHERE organization_id=$1 AND user_id=$2 RETURNING user_id),
 problem AS (UPDATE problems SET owner_id=NULL WHERE id=$3 RETURNING id)
 DELETE FROM problem_members WHERE problem_id=$3 AND user_id=$2`, org.ID, owner.Id, id)
		return err
	}
	data, err := templates.GetBuiltinTemplateZip("builtin:a-plus-b")
	s.Require().NoError(err)
	importer := problems.NewProblemImportUseCase(s.problemsRepo, workspace, transactions)
	_, err = importer.ImportProblemPackage(ctx, bytes.NewReader(data), int64(len(data)), id)
	s.Require().ErrorIs(err, pkg.NoPermission)
	current, err := s.problemsRepo.GetWorkspaceHash(s.ctx, id)
	s.Require().NoError(err)
	s.Equal(previous, current, "a revoked caller must not switch the workspace reference")
}
