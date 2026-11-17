//go:build integration
// +build integration

package integration

import (
	"io"
	"log/slog"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/judging"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

// loadBuiltPackage opens a published package exactly as the judge does.
func (s *IntegrationTestSuite) loadBuiltPackage(id uuid.UUID) *gfmt.Problem {
	return s.loadPackage(id).Format.Problem
}

func (s *IntegrationTestSuite) loadPackage(id uuid.UUID) *judging.LoadedPackage {
	s.T().Helper()
	loader := &judging.PackageLoader{Store: s.objectStore, Bucket: s.bucket, TempDir: diskTempDir(s.T()), Packages: s.problemDrafts, Limits: testBuildOptions(s.T()).Limits}
	pkg, err := loader.LoadPackage(s.ctx, id)
	s.Require().NoError(err)
	s.T().Cleanup(pkg.Cleanup)
	return pkg
}

// Every builtin template builds into a package that names the sources of its
// checker and interactor, and the judge accepts the template's main solution.
func (s *IntegrationTestSuite) TestBuiltinTemplatesBuildAndJudge() {
	admin := s.createUser("admin_roles", accessmodel.UserRoleAdmin)
	org := s.createOrganization("roles-org", "Roles Organization", admin.Id)
	ctx := withTestUser(s.ctx, admin.Id)
	for _, template := range templates.ListBuiltinTemplates() {
		s.Run(template.ID, func() {
			problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: template.ID, OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: template.ID})
			s.Require().NoError(err)
			draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
			s.Require().NoError(err)
			pkg := s.loadPackage(s.buildPackage(admin.Id, problem.ID))
			var main string
			for _, c := range draft.Components {
				if id, ok := draft.MainSolutionID.Get(); ok && c.ID == id {
					main = c.Path
				}
				if c.Role != corev1.ProblemDraftComponentRoleChecker && c.Role != corev1.ProblemDraftComponentRoleInteractor {
					continue
				}
				program, ok := pkg.Format.Problem.Programs[string(c.Role)]
				s.Require().True(ok, "%s missing from package", c.Role)
				s.Equal(gfmt.Program{Source: c.Path, Language: problemsmodel.LanguageForPath(c.Path), Dependencies: map[string]string(c.Dependencies)}, program)
			}
			languages := map[string]submissionsmodel.LanguageName{"cpp": submissionsmodel.Cpp, "python": submissionsmodel.Python, "go": submissionsmodel.Golang}
			language, ok := languages[problemsmodel.LanguageForPath(main)]
			s.Require().True(ok, "main solution %q", main)
			client, files := newGoJudge(s.T())
			cfg := &sandbox.Config{Langs: testBuildOptions(s.T()).Languages}
			judge := &judging.PackageJudge{Client: client, Files: files, Languages: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			judge.Programs = &judging.ProgramCache{Root: diskTempDir(s.T()), Digest: "sha256:" + strings.Repeat("c", 64), Compile: judge.CompileProgram}
			verdict, err := judge.Judge(s.ctx, pkg, uuid.New(), draft.Files[main], language, silentJudgeProgress{}, submissionsmodel.SubmissionEventMeta{})
			s.Require().NoError(err)
			s.Equal(submissionsmodel.Accepted, verdict.State, "%+v", verdict)
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
	s.Equal(gfmt.Program{Source: "checkers/checker.py", Language: "python", Dependencies: map[string]string(checker.Dependencies)}, built.Programs["checker"])
}
