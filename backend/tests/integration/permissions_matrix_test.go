//go:build integration
// +build integration

package integration

import (
	"context"
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
			ShortName:      "unstarted-" + suffix,
			Description:    "Contest that has not started yet",
			StartTime:      &futureStart,
			EndTime:        &futureEnd,
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
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
			resp, err := s.client.GetContestProblemWithResponse(s.ctx, contestID, uuid.New(), func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", partUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Moderator can view problem of unstarted contest", func() {
			resp, err := s.client.GetContestProblemWithResponse(s.ctx, contestID, uuid.New(), func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", modUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			// Problem ID does not exist, so middleware passes (allowing access) and handler returns 404/internal error
			s.NotEqual(http.StatusForbidden, resp.StatusCode())
			s.NotEqual(http.StatusUnauthorized, resp.StatusCode())
		})

		s.Run("Owner can view problem of unstarted contest", func() {
			resp, err := s.client.GetContestProblemWithResponse(s.ctx, contestID, uuid.New(), func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.NotEqual(http.StatusForbidden, resp.StatusCode())
			s.NotEqual(http.StatusUnauthorized, resp.StatusCode())
		})

		s.Run("Admin can view problem of unstarted contest", func() {
			resp, err := s.client.GetContestProblemWithResponse(s.ctx, contestID, uuid.New(), func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", adminUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.NotEqual(http.StatusForbidden, resp.StatusCode())
			s.NotEqual(http.StatusUnauthorized, resp.StatusCode())
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
			ShortName:      "del-target-" + suffix,
			Description:    "Contest deletion test",
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
		})
		s.Require().NoError(err)

		s.Run("Regular non-owner user cannot delete contest", func() {
			resp, err := s.client.DeleteContestWithResponse(s.ctx, contestID, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", otherUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Contest Owner can delete contest", func() {
			resp, err := s.client.DeleteContestWithResponse(s.ctx, contestID, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusOK, resp.StatusCode())
		})
	})

	// 3. Organization Mutation Matrix
	s.Run("Organization Operations Matrix", func() {
		newTitle := "Updated Title " + suffix
		updateBody := corev1.UpdateOrganizationJSONRequestBody{
			Name: &newTitle,
		}

		s.Run("Non-member cannot update organization", func() {
			resp, err := s.client.UpdateOrganizationWithResponse(s.ctx, org.ID, updateBody, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", otherUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Org Owner can update organization", func() {
			resp, err := s.client.UpdateOrganizationWithResponse(s.ctx, org.ID, updateBody, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusOK, resp.StatusCode())
		})

		s.Run("Non-owner cannot delete organization", func() {
			resp, err := s.client.DeleteOrganizationWithResponse(s.ctx, org.ID, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", otherUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Org Owner can delete organization", func() {
			resp, err := s.client.DeleteOrganizationWithResponse(s.ctx, org.ID, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusOK, resp.StatusCode())
		})
	})
}
