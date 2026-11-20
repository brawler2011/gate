//go:build integration
// +build integration

package integration

import (
	"bytes"
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
	ht "github.com/ogen-go/ogen/http"
)

func (s *IntegrationTestSuite) loadBuiltPackage(id uuid.UUID) *gfmt.Problem {
	return s.loadPackage(id).Format.Problem
}

func (s *IntegrationTestSuite) loadPackage(id uuid.UUID) *judging.LoadedPackage {
	s.T().Helper()
	cache, _ := judgeCache(s.T())
	loader := &judging.PackageLoader{Store: s.objectStore, Bucket: s.bucket, Cache: cache, Packages: s.problemDrafts, Limits: testBuildOptions(s.T()).Limits}
	pkg, err := loader.LoadPackage(s.ctx, id)
	s.Require().NoError(err)
	s.T().Cleanup(pkg.Release)
	return pkg
}

// Exported builtin templates import back, publish their checker/interactor sources and accept the main solution.
func (s *IntegrationTestSuite) TestBuiltinTemplatesBuildAndJudge() {
	admin := s.createUser("admin_roles", accessmodel.UserRoleAdmin)
	org := s.createOrganization("roles-org", "Roles Organization", admin.Id)
	ctx := withTestUser(s.ctx, admin.Id)
	for _, template := range templates.ListBuiltinTemplates() {
		s.Run(template.ID, func() {
			source, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: template.ID, OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: template.ID})
			s.Require().NoError(err)
			before, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: source.ID})
			s.Require().NoError(err)
			exported, err := s.client.ExportProblemDraft(ctx, corev1.ExportProblemDraftParams{ID: source.ID})
			s.Require().NoError(err)
			archive, err := io.ReadAll(exported.Data)
			s.Require().NoError(err)
			after, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: source.ID})
			s.Require().NoError(err)
			s.Require().Equal(before.Revision, after.Revision, "export must not change the draft")

			problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: template.ID, OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
			s.Require().NoError(err)
			current, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
			s.Require().NoError(err)
			draft, err := s.client.ImportProblem(ctx, &corev1.ImportProblemReq{Package: ht.MultipartFile{Name: "draft.zip", File: bytes.NewReader(archive), Size: int64(len(archive))}},
				corev1.ImportProblemParams{ID: problem.ID, ExpectedRevision: current.Revision, IdempotencyKey: uuid.NewString()})
			s.Require().NoError(err)
			pkg := s.loadPackage(s.buildPackage(admin.Id, problem.ID))
			languages := map[string]string{}
			if checker, ok := draft.Checker.Get(); ok {
				languages["checker"] = checker.Language
			}
			if interactor, ok := draft.Interactor.Get(); ok {
				languages["interactor"] = interactor.Language
			}
			for role, language := range languages {
				built, ok := pkg.Format.Problem.Programs[role]
				s.Require().True(ok, "%s missing from package", role)
				s.Equal(language, built.Language)
			}
			var main corev1.ProblemDraftSolution
			for _, solution := range draft.Solutions {
				if name, ok := draft.MainSolution.Get(); ok && solution.Name == name {
					main = solution
				}
			}
			language := main.Language
			client, files := newGoJudge(s.T())
			cfg := &sandbox.Config{Langs: testBuildOptions(s.T()).Languages}
			judge := &judging.PackageJudge{Client: client, Files: files, Languages: cfg, Programs: sandbox.NewProgramCache(diskCache(s.T()), client, files, sandbox.Image{Config: cfg}), WorkDir: sandbox.SharedDir{Local: shared.judgeDir, Sandbox: judgeMount}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			verdict, err := judge.Judge(s.ctx, pkg, uuid.New(), main.Source, language, silentJudgeProgress{}, submissionsmodel.SubmissionEventMeta{})
			s.Require().NoError(err)
			s.Equal(submissionsmodel.Accepted, verdict.State, "%+v", verdict)
		})
	}
}

// Changing checker language preserves the component and builds it with its library.
func (s *IntegrationTestSuite) TestCheckerLanguageChangeKeepsComponent() {
	admin := s.createUser("admin_rename", accessmodel.UserRoleAdmin)
	org := s.createOrganization("rename-org", "Rename Organization", admin.Id)
	ctx := withTestUser(s.ctx, admin.Id)
	problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: "Rename", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
	s.Require().NoError(err)

	s.Require().False(draft.Checker.Null)
	s.Require().NotEqual("python3", draft.Checker.Value.Language)
	draft.Checker.Value.Language = "python3"
	draft.Checker.Value.Source = "import sys\nout = open(sys.argv[2]).read().split()\nans = open(sys.argv[3]).read().split()\nsys.exit(0 if out == ans else 1)\n"
	saved, err := s.client.SaveProblemDraft(ctx, draft, corev1.SaveProblemDraftParams{ID: problem.ID, ExpectedRevision: draft.Revision, IdempotencyKey: uuid.NewString()})
	s.Require().NoError(err)
	s.Require().Equal(draft.Tests, saved.Tests)

	built := s.loadBuiltPackage(s.buildPackage(admin.Id, problem.ID))
	program, ok := built.Programs["checker"]
	s.Require().True(ok, "package has the checker")
	s.Equal("python3", program.Language)
	s.True(strings.HasSuffix(program.Source, ".py"), "checker source follows the language: %s", program.Source)
}
