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
	ht "github.com/ogen-go/ogen/http"
)

// Author packages import and build with the real judge.
func (s *IntegrationTestSuite) TestAuthorPackagesImportAndBuild() {
	packages, err := filepath.Glob("../../pkg/testdata/*/*")
	s.Require().NoError(err)
	s.Require().NotEmpty(packages)
	for _, dir := range packages {
		name := filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
		if name == "pcms2/interactive-guess" {
			continue // no main solution to interact with; see the draft import test
		}
		if name == "gfmt/a-plus-b" {
			continue // the builtin template, which TestBuiltinTemplatesBuildAndJudge builds
		}
		// Builds claim the next queued build, so each package gets its own database.
		runIsolated(s.T(), name, func(s *IntegrationTestSuite) {
			admin := s.createUser("admin_import", accessmodel.UserRoleAdmin)
			org := s.createOrganization("import-org", "Import Organization", admin.Id)
			ctx := withTestUser(s.ctx, admin.Id)
			var archive bytes.Buffer
			w := zip.NewWriter(&archive)
			s.Require().NoError(w.AddFS(os.DirFS(dir)))
			s.Require().NoError(w.Close())
			problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{Title: name, OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
			s.Require().NoError(err)
			current, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problem.ID})
			s.Require().NoError(err)
			draft, err := s.client.ImportProblem(ctx, &corev1.ImportProblemReq{Package: ht.MultipartFile{Name: "package.zip", File: bytes.NewReader(archive.Bytes()), Size: int64(archive.Len())}},
				corev1.ImportProblemParams{ID: problem.ID, ExpectedRevision: current.Revision})
			s.Require().NoError(err)
			pkg := s.loadPackage(s.buildPackage(admin.Id, problem.ID))
			built := pkg.Format.Problem
			if filepath.Base(filepath.Dir(dir)) == "polygon" {
				s.requirePolygonInputs(dir, pkg.Format)
			}
			role := "checker"
			if draft.Type == corev1.ProblemTypeInteractive {
				role = "interactor"
			}
			_, _, ok := built.Program(role)
			s.True(ok, "package has no %s", role)
			// A derived limit is a multiple of the rule's resolution; the TLE margin check passed with the build.
			if rule, ok := draft.TimeRule.Get(); ok && draft.TimeLimitMs.Null {
				s.Positive(built.Limits.TimeMs)
				s.Zero(int64(built.Limits.TimeMs)%rule.ResolutionMs, "derived limit %d ms", built.Limits.TimeMs)
			}
		})
	}
}

// Built inputs match the Polygon package, including generated tests.
func (s *IntegrationTestSuite) requirePolygonInputs(dir string, built *gfmt.Built) {
	for _, gt := range built.Problem.Tests() {
		test := gt.Test
		want, err := os.ReadFile(filepath.Join(dir, "tests", fmt.Sprintf("%02d", test.Ordinal)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		s.Require().NoError(err)
		got, err := os.ReadFile(filepath.Join(built.Path, gfmt.TestFolder, test.Input))
		s.Require().NoError(err)
		// The import converts CRLF line endings of text files.
		s.Equal(strings.ReplaceAll(string(want), "\r\n", "\n"), string(got), "input of test %d", test.Ordinal)
	}
}
