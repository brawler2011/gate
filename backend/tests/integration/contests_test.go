//go:build integration
// +build integration

package integration

import (
	"fmt"
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListContests() {
	// 1. Create User
	user := s.createUser("testuser_contests", accessmodel.UserRoleUser)

	// 2. Create Organization
	org := s.createOrganization("test-org", "Test Organization", user.Id)

	// 3. Create Contest
	contestID := uuid.New()
	err := s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: org.ID,
		OwnerID:        &user.Id,
		Title:          "Test Contest",
		Login:          "test-contest",
		Description:    "A test contest",
		Visibility:     contestsmodel.ContestVisibilityPublic,
		Settings:       make(map[string]interface{}),
	})
	s.Require().NoError(err)

	// 4. Update to Public
	visibility := contestsmodel.ContestVisibilityPublic
	err = s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{
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

func (s *IntegrationTestSuite) TestListPublicContestsPagination() {
	user := s.createUser("contest_paged_user_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("contest-paged-org-"+uuid.NewString()[:8], "Contest Paged Org", user.Id)

	prefix := "page-pub-" + uuid.NewString()[:6]
	for i := range 3 {
		id := uuid.New()
		err := s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
			ID:             id,
			OrganizationID: org.ID,
			OwnerID:        &user.Id,
			Title:          fmt.Sprintf("%s-%d", prefix, i),
			Login:          fmt.Sprintf("%s-%d", prefix, i),
			Description:    "Public contest pagination test",
			Visibility:     contestsmodel.ContestVisibilityPublic,
			Settings:       make(map[string]interface{}),
		})
		s.Require().NoError(err)
	}

	for _, invalid := range []corev1.ListPublicContestsParams{
		{Page: corev1.NewOptInt32(0)},
		{PageSize: corev1.NewOptInt32(0)},
		{PageSize: corev1.NewOptInt32(101)},
	} {
		_, err := s.client.ListPublicContests(withTestUser(s.ctx, user.Id), invalid)
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	}

	resp, err := s.client.ListPublicContests(withTestUser(s.ctx, user.Id), corev1.ListPublicContestsParams{
		Search:   corev1.NewOptString(prefix),
		Page:     corev1.NewOptInt32(1),
		PageSize: corev1.NewOptInt32(2),
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Len(resp.Contests, 2)
}

func (s *IntegrationTestSuite) TestClarificationsLifecycle() {
	owner := s.createUser("clar_owner_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	participant := s.createUser("clar_part_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("clar-org-"+uuid.NewString()[:8], "Clar Org", owner.Id)

	contestID := uuid.New()
	login := "clar-contest-" + uuid.NewString()[:6]
	err := s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: org.ID,
		OwnerID:        &owner.Id,
		Title:          "Clar Contest",
		Login:          login,
		Visibility:     contestsmodel.ContestVisibilityPublic,
		Settings:       make(map[string]interface{}),
	})
	s.Require().NoError(err)

	err = s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID,
		UserId:    participant.Id,
		Role:      contestsmodel.ContestRoleParticipant,
	})
	s.Require().NoError(err)

	clar, err := s.client.CreateContestClarification(withTestUser(s.ctx, participant.Id), &corev1.CreateContestClarificationRequestModel{
		Question: "Is the problem statement correct?",
	}, corev1.CreateContestClarificationParams{
		OrgLogin:     org.Login,
		ContestLogin: login,
	})
	s.Require().NoError(err)
	s.Require().NotNil(clar)
	s.Equal("Is the problem statement correct?", clar.Question)

	listResp, err := s.client.ListContestClarifications(withTestUser(s.ctx, owner.Id), corev1.ListContestClarificationsParams{
		OrgLogin:     org.Login,
		ContestLogin: login,
	})
	s.Require().NoError(err)
	s.Require().NotNil(listResp)
	s.Require().Len(listResp.Clarifications, 1)

	answered, err := s.client.AnswerContestClarification(withTestUser(s.ctx, owner.Id), &corev1.AnswerContestClarificationRequestModel{
		Answer: "Yes, it is correct.",
	}, corev1.AnswerContestClarificationParams{
		OrgLogin:        org.Login,
		ContestLogin:    login,
		ClarificationID: clar.ID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(answered)
	s.True(answered.Answer.IsSet())
	s.Equal("Yes, it is correct.", answered.Answer.Value)
}
