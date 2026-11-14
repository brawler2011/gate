//go:build integration
// +build integration

package integration

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
)

// A statement image travels from an imported draft through a real build into
// the package a contest uses, and is served from there.
func (s *IntegrationTestSuite) TestContestProblemAssetServesPackageImage() {
	owner := s.createUser("asset_owner", accessmodel.UserRoleAdmin)
	org := s.createOrganization("asset-org", "Asset Organization", owner.Id)
	ctx := withTestUser(s.ctx, owner.Id)

	created, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(),
		Title: "Asset problem", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	problemID := created.ID

	image := []byte("\x89PNG\r\n\x1a\nstatement image")
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	s.Require().NoError(w.AddFS(os.DirFS("../../pkg/testdata/gfmt/a-plus-b")))
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
	packageID := s.buildPackage(owner.Id, problemID)

	ownerID := owner.Id
	contestID := uuid.New()
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
		ID: contestID, OrganizationID: org.ID, OwnerID: &ownerID, Visibility: contestsmodel.ContestVisibilityPrivate,
		Title: "Asset contest", Login: "asset-contest", Settings: map[string]interface{}{},
	}))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{ContestId: contestID, ProblemId: problemID, PackageId: packageID}))

	params := func(path string) corev1.GetContestProblemAssetParams {
		return corev1.GetContestProblemAssetParams{OrgLogin: org.Login, ContestLogin: "asset-contest", ProblemID: problemID, Path: path}
	}

	resp, err := s.client.GetContestProblemAsset(ctx, params("assets/pic.png"))
	s.Require().NoError(err)
	s.Equal("image/png", resp.ContentType)
	s.Contains(resp.ContentSecurityPolicy.Or(""), "sandbox")
	body, err := io.ReadAll(resp.Response.Data)
	s.Require().NoError(err)
	s.Equal(image, body)

	for _, path := range []string{"assets/missing.png", "statements/en.md", "solutions/main.cpp"} {
		_, err = s.client.GetContestProblemAsset(ctx, params(path))
		s.Equal(http.StatusNotFound, s.getStatusCode(err), path)
	}

	outsider := s.createUser("asset_outsider", accessmodel.UserRoleUser)
	_, err = s.client.GetContestProblemAsset(withTestUser(s.ctx, outsider.Id), params("assets/pic.png"))
	s.Require().Error(err)
	s.NotEqual(http.StatusOK, s.getStatusCode(err))
}
