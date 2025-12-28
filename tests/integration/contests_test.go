package integration

import (
	"context"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListContests() {
	// 1. Create User
	userID := uuid.New()
	kratosID := uuid.New()
	user := models.User{
		Id:       userID,
		Username: "testuser",
		Role:     models.UserRoleUser,
		KratosID: kratosID,
		Email:    "test@example.com",
	}
	err := s.usersRepo.CreateUser(s.ctx, models.CreateUserParams{
		Id:       user.Id,
		Username: user.Username,
		Role:     user.Role,
		KratosId: user.KratosID,
		Email:    user.Email,
	})
	s.Require().NoError(err)

	// 2. Create Contest
	contestID := uuid.New()
	err = s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
		Id:     contestID,
		Title:  "Test Contest",
		UserId: userID,
	})
	s.Require().NoError(err)

	// Update to Public
	visibility := models.ContestVisibilityPublic
	err = s.contestsRepo.UpdateContest(s.ctx, models.ContestUpdateParams{
		Id:         contestID,
		Visibility: &visibility,
	})
	s.Require().NoError(err)

	// 3. Make Request
	resp, err := s.client.ListPublicContestsWithResponse(s.ctx, &corev1.ListPublicContestsParams{
		Page:     1,
		PageSize: 10,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", user.Id.String())
		return nil
	})
	s.Require().NoError(err)

	// 4. Assert Response
	s.Equal(http.StatusOK, resp.StatusCode())
	s.NotNil(resp.JSON200)
	s.Len(resp.JSON200.Contests, 1)
	s.Equal(contestID, resp.JSON200.Contests[0].Id)
	s.Equal("Test Contest", resp.JSON200.Contests[0].Title)
}
