package integration

import (
	"context"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func (s *IntegrationTestSuite) TestProblems() {
	admin := s.createUser("admin_problems", models.UserRoleAdmin)
	// user := s.createUser("user_problems", models.UserRoleUser)

	var problemID uuid.UUID

	// 1. Create Problem (Admin)
	s.Run("CreateProblem", func() {
		// Expect Pandoc call
		s.mockPandoc.EXPECT().BatchConvertLatexToHtml5(gomock.Any(), gomock.Any()).Return([]string{"html content"}, nil).AnyTimes()

		title := "Test Problem"
		resp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
			Title: title,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
		s.NotNil(resp.JSON200)
		problemID = resp.JSON200.Id
	})

	// 2. Get Problem (Admin)
	s.Run("GetProblem", func() {
		resp, err := s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
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
