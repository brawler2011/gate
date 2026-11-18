//go:build integration
// +build integration

package integration

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
	"golang.org/x/text/encoding/charmap"
)

// Every author package in pkg/testdata imports through the API and builds on
// the real judge: its programs compile with their library, generators and the
// main solution run, and the package holds the checker or interactor.
func (s *IntegrationTestSuite) TestAuthorPackagesImportAndBuild() {
	admin := s.createUser("admin_import", accessmodel.UserRoleAdmin)
	org := s.createOrganization("import-org", "Import Organization", admin.Id)
	ctx := withTestUser(s.ctx, admin.Id)
	packages, err := filepath.Glob("../../pkg/testdata/*/*")
	s.Require().NoError(err)
	s.Require().NotEmpty(packages)
	for _, dir := range packages {
		name := filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
		if name == "pcms2/interactive-guess" {
			continue // no main solution to interact with; see the draft import test
		}
		s.Run(name, func() {
			converted := []string{}
			if name == "pcms2/a-plus-b" || name == "polygon/a-plus-b" {
				original := dir
				dir = s.T().TempDir()
				s.Require().NoError(os.CopyFS(dir, os.DirFS(original)))
				path := "check.cpp"
				if name == "polygon/a-plus-b" {
					path = "statements/russian/problem.tex"
					manifest, err := os.ReadFile(filepath.Join(dir, "problem.xml"))
					s.Require().NoError(err)
					manifest = bytes.ReplaceAll(manifest, []byte(`charset="UTF-8" language="russian"`), []byte(`charset="windows-1251" language="russian"`))
					s.Require().NoError(os.WriteFile(filepath.Join(dir, "problem.xml"), manifest, 0o600))
				}
				text, err := os.ReadFile(filepath.Join(dir, path))
				s.Require().NoError(err)
				if name == "pcms2/a-plus-b" {
					text = append([]byte("// Проверка ответа\n"), text...)
				}
				encoded, err := charmap.Windows1251.NewEncoder().Bytes(text)
				s.Require().NoError(err)
				s.Require().NoError(os.WriteFile(filepath.Join(dir, path), encoded, 0o600))
				converted = []string{path}
			}
			var archive bytes.Buffer
			w := zip.NewWriter(&archive)
			s.Require().NoError(w.AddFS(os.DirFS(dir)))
			s.Require().NoError(w.Close())
			problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: name, OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
			s.Require().NoError(err)
			current, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
			s.Require().NoError(err)
			importParams := corev1.ImportProblemParams{ID: problem.ID, ExpectedRevision: current.Revision, IdempotencyKey: uuid.NewString()}
			draft, err := s.client.ImportProblem(ctx, &corev1.ImportProblemReq{Package: ht.MultipartFile{Name: "package.zip", File: bytes.NewReader(archive.Bytes()), Size: int64(archive.Len())}},
				importParams)
			s.Require().NoError(err)
			s.Equal(converted, draft.ConvertedFiles)
			if name == "pcms2/a-plus-b" {
				s.Contains(draft.Draft.Checker.Value.Source, "Проверка ответа")
			}
			if name == "polygon/a-plus-b" {
				readable := false
				for _, statement := range draft.Draft.Statements {
					if statement.Language == "ru" {
						readable = strings.Contains(statement.Legend, "числ")
					}
				}
				s.True(readable, "Russian statement must be readable")
			}
			replay, err := s.client.ImportProblem(ctx, &corev1.ImportProblemReq{Package: ht.MultipartFile{Name: "package.zip", File: bytes.NewReader(archive.Bytes()), Size: int64(archive.Len())}}, importParams)
			s.Require().NoError(err)
			s.Equal(draft, replay)
			built := s.loadBuiltPackage(s.buildPackage(admin.Id, problem.ID))
			role := "checker"
			if draft.Draft.Type == corev1.ProblemDraftTypeInteractive {
				role = "interactor"
			}
			_, ok := built.Programs[role]
			s.True(ok, "package has no %s", role)
		})
	}
}
