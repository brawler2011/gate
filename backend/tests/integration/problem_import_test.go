//go:build integration
// +build integration

package integration

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
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
			var archive bytes.Buffer
			w := zip.NewWriter(&archive)
			s.Require().NoError(w.AddFS(os.DirFS(dir)))
			s.Require().NoError(w.Close())
			problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: name, OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
			s.Require().NoError(err)
			current, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
			s.Require().NoError(err)
			draft, err := s.client.ImportProblem(ctx, &corev1.ImportProblemReq{Package: ht.MultipartFile{Name: "package.zip", File: bytes.NewReader(archive.Bytes()), Size: int64(archive.Len())}},
				corev1.ImportProblemParams{ID: problem.ID, ExpectedRevision: current.Revision, IdempotencyKey: uuid.NewString()})
			s.Require().NoError(err)
			pkg := s.loadPackage(s.buildPackage(admin.Id, problem.ID))
			built := pkg.Format.Problem
			if filepath.Base(filepath.Dir(dir)) == "polygon" {
				s.requirePolygonInputs(dir, pkg.Format)
			}
			role := "checker"
			if draft.Type == corev1.ProblemDraftTypeInteractive {
				role = "interactor"
			}
			_, ok := built.Programs[role]
			s.True(ok, "package has no %s", role)
		})
	}
}

// requirePolygonInputs checks that every test input of a Polygon package found
// in its tests/ folder, generated ones included, is the input Gate built: the
// generator receives exactly the arguments of the test, as in Polygon.
func (s *IntegrationTestSuite) requirePolygonInputs(dir string, built *gfmt.GateFormat) {
	for _, subtask := range built.Problem.Subtasks {
		for _, test := range subtask.Tests {
			want, err := os.ReadFile(filepath.Join(dir, "tests", fmt.Sprintf("%02d", test.Ordinal)))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			s.Require().NoError(err)
			got, err := os.ReadFile(filepath.Join(built.Path, test.Manual))
			s.Require().NoError(err)
			// The import converts CRLF line endings of text files.
			s.Equal(strings.ReplaceAll(string(want), "\r\n", "\n"), string(got), "input of test %d", test.Ordinal)
		}
	}
}
