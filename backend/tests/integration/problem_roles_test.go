//go:build integration
// +build integration

package integration

import (
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/judging"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
)

// loadBuiltPackage opens a published package exactly as the judge does.
func (s *IntegrationTestSuite) loadBuiltPackage(id uuid.UUID) *gfmt.Problem {
	s.T().Helper()
	loader := &judging.PackageLoader{Store: s.objectStore, Bucket: s.bucket, TempDir: diskTempDir(s.T()), Packages: s.problemDrafts, Limits: testBuildOptions(s.T()).Limits}
	pkg, err := loader.LoadPackage(s.ctx, id)
	s.Require().NoError(err)
	s.T().Cleanup(pkg.Cleanup)
	return pkg.Format.Problem
}

// Every builtin template builds, and its package holds the single checker,
// validator and interactor its role folders define.
func (s *IntegrationTestSuite) TestBuiltinTemplatesBuildWithSingletonRoles() {
	admin := s.createUser("admin_roles", accessmodel.UserRoleAdmin)
	org := s.createOrganization("roles-org", "Roles Organization", admin.Id)
	ctx := withTestUser(s.ctx, admin.Id)
	for _, template := range templates.ListBuiltinTemplates() {
		s.Run(template.ID, func() {
			problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: template.ID, OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: template.ID})
			s.Require().NoError(err)
			draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
			s.Require().NoError(err)
			built := s.loadBuiltPackage(s.buildPackage(admin.Id, problem.ID))
			for _, c := range draft.Components {
				if c.Role != corev1.ProblemDraftComponentRoleChecker && c.Role != corev1.ProblemDraftComponentRoleValidator && c.Role != corev1.ProblemDraftComponentRoleInteractor {
					continue
				}
				compiled, ok := built.CompiledRole(string(c.Role))
				s.Require().True(ok, "%s missing from package", c.Role)
				s.Equal(built.Compiled[c.ID.String()], compiled)
			}
			role := "checker"
			if draft.Type == corev1.ProblemDraftTypeInteractive {
				role = "interactor"
			}
			_, ok := built.CompiledRole(role)
			s.True(ok, "package has no %s", role)
		})
	}
}

// Changing the checker language renames its file; the build compiles it in the
// new language under the same component ID and dependencies.
func (s *IntegrationTestSuite) TestCheckerLanguageChangeKeepsComponent() {
	admin := s.createUser("admin_rename", accessmodel.UserRoleAdmin)
	org := s.createOrganization("rename-org", "Rename Organization", admin.Id)
	ctx := withTestUser(s.ctx, admin.Id)
	problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: "Rename", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
	s.Require().NoError(err)

	index := -1
	for i, c := range draft.Components {
		if c.Role == corev1.ProblemDraftComponentRoleChecker {
			index = i
		}
	}
	s.Require().NotEqual(-1, index)
	checker := draft.Components[index]
	s.Require().Equal("checkers/checker.cpp", checker.Path)
	s.Require().NotEmpty(checker.Dependencies)
	delete(draft.Files, checker.Path)
	draft.Files["checkers/checker.py"] = "import sys\nout = open(sys.argv[2]).read().split()\nans = open(sys.argv[3]).read().split()\nsys.exit(0 if out == ans else 1)\n"
	draft.Components[index].Path = "checkers/checker.py"
	saved, err := s.client.SaveProblemDraft(ctx, draft, corev1.SaveProblemDraftParams{ID: problem.ID, ExpectedRevision: draft.Revision, IdempotencyKey: uuid.NewString()})
	s.Require().NoError(err)
	s.Require().Equal(draft.Tests, saved.Tests)

	built := s.loadBuiltPackage(s.buildPackage(admin.Id, problem.ID))
	compiled, ok := built.Compiled[checker.ID.String()]
	s.Require().True(ok, "checker keeps its component ID")
	s.Equal("checker", compiled.Role)
	s.Equal("python", compiled.Language)
	s.Equal(map[string]string(checker.Dependencies), built.Sources["checkers/checker.py"].DependencyMap)
	for path := range built.Sources {
		s.False(strings.HasSuffix(path, "checker.cpp"), "old checker source remains: %s", path)
	}
}
