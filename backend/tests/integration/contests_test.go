//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListContests() {
	// 1. Create User
	user := s.createUser("testuser_contests", models.UserRoleUser)

	// 2. Create Organization
	org := s.createOrganization("test-org", "Test Organization", user.Id)

	// 3. Create Contest
	contestID := uuid.New()
	err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
		ID:             contestID,
		OrganizationID: org.ID,
		OwnerID:        &user.Id,
		Titles:         map[string]string{"en": "Test Contest"},
		ShortName:      "test-contest",
		Description:    "A test contest",
		Visibility:     models.ContestVisibilityPublic,
		Settings:       make(map[string]interface{}),
		AccessPolicy:   make(map[string]interface{}),
	})
	s.Require().NoError(err)

	// 4. Update to Public
	visibility := models.ContestVisibilityPublic
	err = s.contestsRepo.UpdateContest(s.ctx, models.ContestUpdateParams{
		ID:         contestID,
		Visibility: &visibility,
	})
	s.Require().NoError(err)

	// 5. Make Request
	resp, err := s.client.ListPublicContestsWithResponse(s.ctx, &corev1.ListPublicContestsParams{
		Page:     1,
		PageSize: 10,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", user.Id.String())
		return nil
	})
	s.Require().NoError(err)

	// 6. Assert Response
	s.Equal(http.StatusOK, resp.StatusCode())
	s.NotNil(resp.JSON200)
	s.Len(resp.JSON200.Contests, 1)
	s.Equal(contestID, resp.JSON200.Contests[0].Id)
	s.Equal("Test Contest", resp.JSON200.Contests[0].Title)
}
