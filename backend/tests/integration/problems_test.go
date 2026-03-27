//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *IntegrationTestSuite) TestProblems() {
	admin := s.createUser("admin_problems", models.UserRoleAdmin)
	// user := s.createUser("user_problems", models.UserRoleUser)

	org := s.createOrganization("admin-org", "Admin Organization", admin.Id)

	var problemID uuid.UUID

	// 1. Create Problem (Admin)
	s.Run("CreateProblem", func() {
		title := "Test Problem"
		organizationID := openapi_types.UUID(org.ID)
		resp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
			Title:          title,
			OrganizationId: &organizationID,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Require().Equal(http.StatusOK, resp.StatusCode())
		s.Require().NotNil(resp.JSON200)
		problemID = resp.JSON200.Id
	})

	// 2. Get Problem (Admin)
	s.Run("GetProblem", func() {
		resp, err := s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Require().Equal(http.StatusOK, resp.StatusCode())
		s.Require().NotNil(resp.JSON200)
		s.Equal(problemID, resp.JSON200.Problem.Id)
	})

	// 3. Update Problem (Admin)
	s.Run("UpdateProblem", func() {
		newTitle := "Updated Problem"
		visibility := "public"
		resp, err := s.client.UpdateProblemWithResponse(s.ctx, problemID, corev1.UpdateProblemJSONRequestBody{
			Title:      &newTitle,
			Visibility: &visibility,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
	})

	// 4. List Problems
	s.Run("ListProblems", func() {
		resp, err := s.client.ListProblemsWithResponse(s.ctx, &corev1.ListProblemsParams{
			Page:     1,
			PageSize: 10,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
		s.GreaterOrEqual(len(resp.JSON200.Problems), 1)
	})

	// 5. Delete Problem (Admin)
	s.Run("DeleteProblem", func() {
		resp, err := s.client.DeleteProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
	})
}
