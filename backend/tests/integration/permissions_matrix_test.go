//go:build integration
// +build integration

package integration

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestPermissionsMatrix() {
	suffix := uuid.NewString()[:8]

	ownerUser := s.createUser("matrix_owner_"+suffix, models.UserRoleUser)
	modUser := s.createUser("matrix_mod_"+suffix, models.UserRoleUser)
	partUser := s.createUser("matrix_part_"+suffix, models.UserRoleUser)
	otherUser := s.createUser("matrix_other_"+suffix, models.UserRoleUser)
	adminUser := s.createUser("matrix_admin_"+suffix, models.UserRoleAdmin)

	org := s.createOrganization("matrix-org-"+suffix, "Matrix Org", ownerUser.Id)

	// 1. Contest Not Started Matrix Tests
	s.Run("Contest Not Started - Role Matrix", func() {
		futureStart := time.Now().Add(24 * time.Hour)
		futureEnd := time.Now().Add(48 * time.Hour)
		contestID := uuid.New()
		ownerID := ownerUser.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "Unstarted Contest",
			Login:          "unstarted-" + suffix,
			Description:    "Contest that has not started yet",
			StartTime:      &futureStart,
			EndTime:        &futureEnd,
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: contestID,
			UserId:    partUser.Id,
			Role:      models.ContestRoleParticipant,
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: contestID,
			UserId:    modUser.Id,
			Role:      models.ContestRoleModerator,
		})
		s.Require().NoError(err)

		s.Run("Participant cannot view problem of unstarted contest", func() {
			_, err := s.client.GetContestProblem(withTestUser(s.ctx, partUser.Id), corev1.GetContestProblemParams{
				OrgLogin:     org.Login,
				ContestLogin: "unstarted-" + suffix,
				ProblemID:    uuid.New(),
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Moderator can view problem of unstarted contest", func() {
			_, err := s.client.GetContestProblem(withTestUser(s.ctx, modUser.Id), corev1.GetContestProblemParams{
				OrgLogin:     org.Login,
				ContestLogin: "unstarted-" + suffix,
				ProblemID:    uuid.New(),
			})
			// Problem ID does not exist, so middleware passes (allowing access) and handler returns 404/internal error
			s.NotEqual(http.StatusForbidden, s.getStatusCode(err))
			s.NotEqual(http.StatusUnauthorized, s.getStatusCode(err))
		})

		s.Run("Owner can view problem of unstarted contest", func() {
			_, err := s.client.GetContestProblem(withTestUser(s.ctx, ownerUser.Id), corev1.GetContestProblemParams{
				OrgLogin:     org.Login,
				ContestLogin: "unstarted-" + suffix,
				ProblemID:    uuid.New(),
			})
			s.NotEqual(http.StatusForbidden, s.getStatusCode(err))
			s.NotEqual(http.StatusUnauthorized, s.getStatusCode(err))
		})

		s.Run("Admin can view problem of unstarted contest", func() {
			_, err := s.client.GetContestProblem(withTestUser(s.ctx, adminUser.Id), corev1.GetContestProblemParams{
				OrgLogin:     org.Login,
				ContestLogin: "unstarted-" + suffix,
				ProblemID:    uuid.New(),
			})
			s.NotEqual(http.StatusForbidden, s.getStatusCode(err))
			s.NotEqual(http.StatusUnauthorized, s.getStatusCode(err))
		})
	})

	// 2. Contest Deletion & Administration Matrix
	s.Run("Contest Deletion Matrix", func() {
		contestID := uuid.New()
		ownerID := ownerUser.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "Delete Target Contest",
			Login:          "del-target-" + suffix,
			Description:    "Contest deletion test",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		s.Run("Regular non-owner user cannot delete contest", func() {
			err := s.client.DeleteContest(withTestUser(s.ctx, otherUser.Id), corev1.DeleteContestParams{
				OrgLogin:     org.Login,
				ContestLogin: "del-target-" + suffix,
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Contest Owner can delete contest", func() {
			err := s.client.DeleteContest(withTestUser(s.ctx, ownerUser.Id), corev1.DeleteContestParams{
				OrgLogin:     org.Login,
				ContestLogin: "del-target-" + suffix,
			})
			s.Require().NoError(err)
		})
	})

	// 3. Organization Mutation Matrix
	s.Run("Organization Operations Matrix", func() {
		updateBody := &corev1.UpdateOrganizationRequestModel{
			Name: corev1.NewOptString("Updated Title " + suffix),
		}

		s.Run("Non-member cannot update organization", func() {
			err := s.client.UpdateOrganization(withTestUser(s.ctx, otherUser.Id), updateBody, corev1.UpdateOrganizationParams{
				Login: org.Login,
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Org Owner can update organization", func() {
			err := s.client.UpdateOrganization(withTestUser(s.ctx, ownerUser.Id), updateBody, corev1.UpdateOrganizationParams{
				Login: org.Login,
			})
			s.Require().NoError(err)
		})

		s.Run("Non-owner cannot delete organization", func() {
			err := s.client.DeleteOrganization(withTestUser(s.ctx, otherUser.Id), corev1.DeleteOrganizationParams{
				Login: org.Login,
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Org Owner can delete organization", func() {
			err := s.client.DeleteOrganization(withTestUser(s.ctx, ownerUser.Id), corev1.DeleteOrganizationParams{
				Login: org.Login,
			})
			s.Require().NoError(err)
		})
	})
}
