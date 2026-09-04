//go:build integration
// +build integration

package integration

import (
	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
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
		Title:          "Test Contest",
		Login:          "test-contest",
		Description:    "A test contest",
		Visibility:     models.ContestVisibilityPublic,
		Settings:       make(map[string]interface{}),
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
	resp, err := s.client.ListPublicContests(withTestUser(s.ctx, user.Id), corev1.ListPublicContestsParams{
		Page:     corev1.NewOptInt32(1),
		PageSize: corev1.NewOptInt32(10),
	})
	s.Require().NoError(err)

	// 6. Assert Response
	s.NotNil(resp)
	s.Len(resp.Contests, 1)
	s.Equal(contestID, resp.Contests[0].ID)
	s.Equal("Test Contest", resp.Contests[0].Title)
}
