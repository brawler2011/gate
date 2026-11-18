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
			for role, program := range map[string]corev1.NilProblemDraftProgram{"checker": draft.Checker, "interactor": draft.Interactor} {
				if program.Null {
					continue
				}
				built, ok := pkg.Format.Problem.Programs[role]
				s.Require().True(ok, "%s missing from package", role)
				s.Equal(program.Value.Language, built.Language)
			}
			var main corev1.ProblemDraftNamedProgram
			for _, solution := range draft.Solutions {
				if id, ok := draft.MainSolutionID.Get(); ok && solution.ID == id {
					main = solution
				}
			}
			languages := map[string]submissionsmodel.LanguageName{"cpp": submissionsmodel.Cpp, "python": submissionsmodel.Python, "go": submissionsmodel.Golang}
			language, ok := languages[main.Language]
			s.Require().True(ok, "main solution %q", main.Name)
			client, files := newGoJudge(s.T())
			cfg := &sandbox.Config{Langs: testBuildOptions(s.T()).Languages}
			judge := &judging.PackageJudge{Client: client, Files: files, Languages: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			judge.Programs = &judging.ProgramCache{Root: diskTempDir(s.T()), Compiler: "test", Compile: judge.CompileProgram}
			verdict, err := judge.Judge(s.ctx, pkg, uuid.New(), main.Source, language, silentJudgeProgress{}, submissionsmodel.SubmissionEventMeta{})
			s.Require().NoError(err)
			s.Equal(submissionsmodel.Accepted, verdict.State, "%+v", verdict)
		})
	}
}

// Changing the checker language keeps its dependencies; the build compiles it
// in the new language and the package file follows the language.
func (s *IntegrationTestSuite) TestCheckerLanguageChangeKeepsComponent() {
	admin := s.createUser("admin_rename", accessmodel.UserRoleAdmin)
	org := s.createOrganization("rename-org", "Rename Organization", admin.Id)
	ctx := withTestUser(s.ctx, admin.Id)
	problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: "Rename", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
	s.Require().NoError(err)

	s.Require().False(draft.Checker.Null)
	s.Require().Equal("cpp", draft.Checker.Value.Language)
	s.Require().NotEmpty(draft.Checker.Value.Dependencies)
	draft.Checker.Value.Language = "python"
	draft.Checker.Value.Source = "import sys\nout = open(sys.argv[2]).read().split()\nans = open(sys.argv[3]).read().split()\nsys.exit(0 if out == ans else 1)\n"
	saved, err := s.client.SaveProblemDraft(ctx, draft, corev1.SaveProblemDraftParams{ID: problem.ID, ExpectedRevision: draft.Revision, IdempotencyKey: uuid.NewString()})
	s.Require().NoError(err)
	s.Require().Equal(draft.Tests, saved.Tests)

	built := s.loadBuiltPackage(s.buildPackage(admin.Id, problem.ID))
	program, ok := built.Programs["checker"]
	s.Require().True(ok, "package has the checker")
	s.Equal("python", program.Language)
	s.True(strings.HasSuffix(program.Source, ".py"), "checker source follows the language: %s", program.Source)
	s.Equal(map[string]string{"testlib.h": "lib/testlib.h"}, program.Dependencies)
}
