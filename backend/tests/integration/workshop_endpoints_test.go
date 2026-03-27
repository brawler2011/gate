//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/problemformat"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *IntegrationTestSuite) TestWorkshopManifestSyncsProblemTitle() {
	admin := s.createUser("workshop_title_admin", models.UserRoleAdmin)
	org := s.createOrganization("workshop-title-org", "Workshop Title Org", admin.Id)

	initialTitle := "Original Workshop Title"
	organizationID := openapi_types.UUID(org.ID)
	createResp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
		Title:          initialTitle,
		OrganizationId: &organizationID,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, createResp.StatusCode())
	s.Require().NotNil(createResp.JSON200)

	problemID := createResp.JSON200.Id

	initResp, err := s.client.InitProblemWorkshopWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Equal(http.StatusOK, initResp.StatusCode())

	manifestResp, err := s.client.GetWorkshopFileWithResponse(s.ctx, problemID, "manifest.json", func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Equal(http.StatusOK, manifestResp.StatusCode())

	var manifest problemformat.ProblemManifest
	err = json.Unmarshal(manifestResp.Body, &manifest)
	s.Require().NoError(err)

	manifest.Statement.Title = "Synced From Manifest"

	manifestJSON, err := json.Marshal(manifest)
	s.Require().NoError(err)

	updateResp, err := s.client.UpdateWorkshopFileWithBodyWithResponse(
		s.ctx,
		problemID,
		"manifest.json",
		"application/octet-stream",
		bytes.NewReader(manifestJSON),
		func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		},
	)
	s.Require().NoError(err)
	s.Equal(http.StatusOK, updateResp.StatusCode())

	problemResp, err := s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Equal(http.StatusOK, problemResp.StatusCode())
	s.Equal("Synced From Manifest", problemResp.JSON200.Problem.Title)
}
