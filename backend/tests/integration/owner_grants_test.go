//go:build integration
// +build integration

package integration

import (
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestOwnerGrantsRestrictedForAdmins() {
	suffix := uuid.NewString()[:8]
	owner := s.createUser("org_owner_"+suffix, accessmodel.UserRoleUser)
	admin := s.createUser("org_admin_"+suffix, accessmodel.UserRoleUser)
	globalAdmin := s.createUser("global_admin_"+suffix, accessmodel.UserRoleAdmin)
	org := s.createOrganization("owner-grants-org-"+suffix, "Owner Grants Org", owner.Id)

	err := s.organizationsRepo.AddMember(s.ctx, org.ID, admin.Id, organizationsmodel.OrgRoleAdmin)
	s.Require().NoError(err)

	s.Run("Direct membership owner grant", func() {
		targetAdmin := s.createUser("target_admin_"+suffix, accessmodel.UserRoleUser)
		targetOwner := s.createUser("target_owner_"+suffix, accessmodel.UserRoleUser)
		targetGlobal := s.createUser("target_global_"+suffix, accessmodel.UserRoleUser)

		// Admin cannot add owner
		err := s.client.AddOrganizationMember(withTestUser(s.ctx, admin.Id), corev1.AddOrganizationMemberParams{
			Login:  org.Login,
			UserID: targetAdmin.Id,
			Role:   "owner",
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))

		// Admin can add member and admin
		err = s.client.AddOrganizationMember(withTestUser(s.ctx, admin.Id), corev1.AddOrganizationMemberParams{
			Login:  org.Login,
			UserID: targetAdmin.Id,
			Role:   "member",
		})
		s.Require().NoError(err)

		// Owner can add owner
		err = s.client.AddOrganizationMember(withTestUser(s.ctx, owner.Id), corev1.AddOrganizationMemberParams{
			Login:  org.Login,
			UserID: targetOwner.Id,
			Role:   "owner",
		})
		s.Require().NoError(err)

		// Global admin can add owner
		err = s.client.AddOrganizationMember(withTestUser(s.ctx, globalAdmin.Id), corev1.AddOrganizationMemberParams{
			Login:  org.Login,
			UserID: targetGlobal.Id,
			Role:   "owner",
		})
		s.Require().NoError(err)
	})

	s.Run("Invitation owner grant", func() {
		targetAdmin := s.createUser("inv_admin_"+suffix, accessmodel.UserRoleUser)
		targetOwner := s.createUser("inv_owner_"+suffix, accessmodel.UserRoleUser)
		targetGlobal := s.createUser("inv_global_"+suffix, accessmodel.UserRoleUser)

		// Admin cannot invite with owner role
		_, err := s.client.InviteOrganizationMember(withTestUser(s.ctx, admin.Id), &corev1.InviteOrganizationMemberRequestModel{
			UserID: targetAdmin.Id,
			Role:   corev1.InviteOrganizationMemberRequestModelRoleOwner,
		}, corev1.InviteOrganizationMemberParams{
			Login: org.Login,
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))

		// Admin can invite with admin role
		_, err = s.client.InviteOrganizationMember(withTestUser(s.ctx, admin.Id), &corev1.InviteOrganizationMemberRequestModel{
			UserID: targetAdmin.Id,
			Role:   corev1.InviteOrganizationMemberRequestModelRoleAdmin,
		}, corev1.InviteOrganizationMemberParams{
			Login: org.Login,
		})
		s.Require().NoError(err)

		// Owner can invite with owner role
		_, err = s.client.InviteOrganizationMember(withTestUser(s.ctx, owner.Id), &corev1.InviteOrganizationMemberRequestModel{
			UserID: targetOwner.Id,
			Role:   corev1.InviteOrganizationMemberRequestModelRoleOwner,
		}, corev1.InviteOrganizationMemberParams{
			Login: org.Login,
		})
		s.Require().NoError(err)

		// Global admin can invite with owner role
		_, err = s.client.InviteOrganizationMember(withTestUser(s.ctx, globalAdmin.Id), &corev1.InviteOrganizationMemberRequestModel{
			UserID: targetGlobal.Id,
			Role:   corev1.InviteOrganizationMemberRequestModelRoleOwner,
		}, corev1.InviteOrganizationMemberParams{
			Login: org.Login,
		})
		s.Require().NoError(err)
	})

	s.Run("Join request approval owner grant", func() {
		targetAdmin := s.createUser("req_admin_"+suffix, accessmodel.UserRoleUser)
		targetOwner := s.createUser("req_owner_"+suffix, accessmodel.UserRoleUser)
		targetGlobal := s.createUser("req_global_"+suffix, accessmodel.UserRoleUser)

		reqAdmin, err := s.organizationsRepo.CreateJoinRequest(s.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{
			OrganizationID: org.ID,
			UserID:         targetAdmin.Id,
		})
		s.Require().NoError(err)

		reqOwner, err := s.organizationsRepo.CreateJoinRequest(s.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{
			OrganizationID: org.ID,
			UserID:         targetOwner.Id,
		})
		s.Require().NoError(err)

		reqGlobal, err := s.organizationsRepo.CreateJoinRequest(s.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{
			OrganizationID: org.ID,
			UserID:         targetGlobal.Id,
		})
		s.Require().NoError(err)

		// Admin cannot approve request with owner role
		err = s.client.ApproveOrganizationJoinRequest(withTestUser(s.ctx, admin.Id), corev1.NewOptApproveOrganizationJoinRequestModel(corev1.ApproveOrganizationJoinRequestModel{
			Role: corev1.NewOptApproveOrganizationJoinRequestModelRole(corev1.ApproveOrganizationJoinRequestModelRoleOwner),
		}), corev1.ApproveOrganizationJoinRequestParams{
			Login: org.Login,
			ID:    reqAdmin.ID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))

		// Admin can approve request with admin role
		err = s.client.ApproveOrganizationJoinRequest(withTestUser(s.ctx, admin.Id), corev1.NewOptApproveOrganizationJoinRequestModel(corev1.ApproveOrganizationJoinRequestModel{
			Role: corev1.NewOptApproveOrganizationJoinRequestModelRole(corev1.ApproveOrganizationJoinRequestModelRoleAdmin),
		}), corev1.ApproveOrganizationJoinRequestParams{
			Login: org.Login,
			ID:    reqAdmin.ID,
		})
		s.Require().NoError(err)

		// Owner can approve request with owner role
		err = s.client.ApproveOrganizationJoinRequest(withTestUser(s.ctx, owner.Id), corev1.NewOptApproveOrganizationJoinRequestModel(corev1.ApproveOrganizationJoinRequestModel{
			Role: corev1.NewOptApproveOrganizationJoinRequestModelRole(corev1.ApproveOrganizationJoinRequestModelRoleOwner),
		}), corev1.ApproveOrganizationJoinRequestParams{
			Login: org.Login,
			ID:    reqOwner.ID,
		})
		s.Require().NoError(err)

		// Global admin can approve request with owner role
		err = s.client.ApproveOrganizationJoinRequest(withTestUser(s.ctx, globalAdmin.Id), corev1.NewOptApproveOrganizationJoinRequestModel(corev1.ApproveOrganizationJoinRequestModel{
			Role: corev1.NewOptApproveOrganizationJoinRequestModelRole(corev1.ApproveOrganizationJoinRequestModelRoleOwner),
		}), corev1.ApproveOrganizationJoinRequestParams{
			Login: org.Login,
			ID:    reqGlobal.ID,
		})
		s.Require().NoError(err)
	})
}
