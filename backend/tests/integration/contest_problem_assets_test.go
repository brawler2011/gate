//go:build integration
// +build integration

package integration

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
)

// importWithImage replaces the problem draft with the a-plus-b package plus a
// statement image, marking one generated test as a sample.
func (s *IntegrationTestSuite) importWithImage(userID, problemID uuid.UUID, image []byte) {
	s.T().Helper()
	ctx := withTestUser(s.ctx, userID)
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	root := os.DirFS("../../pkg/testdata/gfmt/a-plus-b")
	s.Require().NoError(fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		if path == "problem.yaml" {
			data = []byte(strings.Replace(string(data), `- generate: "echo 10 20"`, "- generate: \"echo 10 20\"\n        sample: true", 1))
		}
		f, err := w.Create(path)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}))
	f, err := w.Create("assets/pic.png")
	s.Require().NoError(err)
	_, err = f.Write(image)
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problemID})
	s.Require().NoError(err)
	_, err = s.client.ImportProblem(ctx, &corev1.ImportProblemReq{Package: ht.MultipartFile{Name: "package.zip", File: bytes.NewReader(archive.Bytes()), Size: int64(archive.Len())}},
		corev1.ImportProblemParams{ID: problemID, ExpectedRevision: draft.Revision, IdempotencyKey: uuid.NewString()})
	s.Require().NoError(err)
}

// removePackageArchive deletes the archive of a built package from object
// storage, so any statement read that still opens it fails.
func (s *IntegrationTestSuite) removePackageArchive(packageID uuid.UUID) {
	s.T().Helper()
	descriptor, err := s.problemDrafts.GetPackageDescriptor(s.ctx, packageID)
	s.Require().NoError(err)
	s.Require().NoError(s.objectStore.DeleteFile(s.ctx, s.bucket, descriptor.Object.Key))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Statement images and samples travel from an imported draft through a real
// build into the package a contest uses, and are served without its archive.
func (s *IntegrationTestSuite) TestContestProblemAssetServesPackageImage() {
	owner := s.createUser("asset_owner", accessmodel.UserRoleAdmin)
	org := s.createOrganization("asset-org", "Asset Organization", owner.Id)
	ctx := withTestUser(s.ctx, owner.Id)

	created, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(),
		Title: "Asset problem", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	problemID := created.ID

	image := []byte("\x89PNG\r\n\x1a\nstatement image")
	s.importWithImage(owner.Id, problemID, image)
	packageID := s.buildPackage(owner.Id, problemID)
	s.removePackageArchive(packageID)

	ownerID := owner.Id
	contestID := uuid.New()
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
		ID: contestID, OrganizationID: org.ID, OwnerID: &ownerID, Visibility: contestsmodel.ContestVisibilityPrivate,
		Title: "Asset contest", Login: "asset-contest", Settings: map[string]interface{}{},
	}))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: problemID, PackageId: packageID}))

	problem := func() corev1.ContestProblemModel {
		resp, err := s.client.GetContestProblem(ctx, corev1.GetContestProblemParams{OrgLogin: org.Login, ContestLogin: "asset-contest", ProblemID: problemID})
		s.Require().NoError(err)
		return resp.Problem
	}
	params := func(hash string) corev1.GetContestProblemAssetParams {
		return corev1.GetContestProblemAssetParams{OrgLogin: org.Login, ContestLogin: "asset-contest", ProblemID: problemID, SHA256: hash}
	}

	got := problem()
	s.Equal(corev1.ContestProblemModelAssets{"assets/pic.png": sha256Hex(image)}, got.Assets)
	s.Len(got.Samples, 4, "three manual samples and one generated")
	s.Contains(got.Samples, corev1.ProblemSampleModel{Input: "10 20\n", Output: "30\n"}, "generated sample input")
	s.Contains(got.Samples, corev1.ProblemSampleModel{Input: "7 8\n", Output: "15\n"}, "manual sample input")

	resp, err := s.client.GetContestProblemAsset(ctx, params(sha256Hex(image)))
	s.Require().NoError(err)
	s.Equal("image/png", resp.ContentType)
	s.Equal("private, max-age=31536000, immutable", resp.CacheControl.Or(""))
	s.Equal(`"`+sha256Hex(image)+`"`, resp.ETag.Or(""))
	s.Contains(resp.ContentSecurityPolicy.Or(""), "sandbox")
	body, err := io.ReadAll(resp.Response.Data)
	s.Require().NoError(err)
	s.Equal(image, body)

	// A stored test input is an object of the package build, not a statement asset.
	for _, hash := range []string{sha256Hex([]byte("missing")), sha256Hex([]byte("7 8\n"))} {
		_, err = s.client.GetContestProblemAsset(ctx, params(hash))
		s.Equal(http.StatusNotFound, s.getStatusCode(err), hash)
	}

	outsider := s.createUser("asset_outsider", accessmodel.UserRoleUser)
	_, err = s.client.GetContestProblemAsset(withTestUser(s.ctx, outsider.Id), params(sha256Hex(image)))
	s.Require().Error(err)
	s.NotEqual(http.StatusOK, s.getStatusCode(err))

	// Switching to a package with a changed image changes its URL at once, and
	// the old hash is no longer served for this contest problem.
	changed := []byte("\x89PNG\r\n\x1a\nchanged image")
	s.importWithImage(owner.Id, problemID, changed)
	next := s.buildPackage(owner.Id, problemID)
	s.removePackageArchive(next)
	s.Require().NoError(s.contestsRepo.UpdateContestProblemPackage(s.ctx, contestID, problemID, next))
	s.Equal(corev1.ContestProblemModelAssets{"assets/pic.png": sha256Hex(changed)}, problem().Assets)
	resp, err = s.client.GetContestProblemAsset(ctx, params(sha256Hex(changed)))
	s.Require().NoError(err)
	body, err = io.ReadAll(resp.Response.Data)
	s.Require().NoError(err)
	s.Equal(changed, body)
	_, err = s.client.GetContestProblemAsset(ctx, params(sha256Hex(image)))
	s.Equal(http.StatusNotFound, s.getStatusCode(err))
}
