//go:build integration

package integration

import (
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
