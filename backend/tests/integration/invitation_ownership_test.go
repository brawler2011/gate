//go:build integration

package integration

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
)

func (s *IntegrationTestSuite) TestInvitationResponseUsesAuthenticatedRecipient() {
	owner := s.createUser("invitation_owner", accessmodel.UserRoleUser)
	recipient := s.createUser("invitation_recipient", accessmodel.UserRoleUser)
	outsider := s.createUser("invitation_outsider", accessmodel.UserRoleUser)
	org := s.createOrganization("invitation-response", "Invitation response", owner.Id)
	for _, operation := range []string{"accept", "decline"} {
		s.Run(operation, func() {
			invitation, err := s.organizationsRepo.CreateInvitation(s.ctx, &organizationsmodel.CreateOrganizationInvitationInput{OrganizationID: org.ID, UserID: recipient.Id, InviterID: owner.Id, Role: organizationsmodel.OrgRoleMember})
			s.Require().NoError(err)
			call := func(user accessmodel.User) error {
				if operation == "accept" {
					return s.client.AcceptOrganizationInvitation(withTestUser(s.ctx, user.Id), corev1.AcceptOrganizationInvitationParams{ID: invitation.ID})
				}
				return s.client.DeclineOrganizationInvitation(withTestUser(s.ctx, user.Id), corev1.DeclineOrganizationInvitationParams{ID: invitation.ID})
			}
			s.Require().Error(call(outsider))
			saved, err := s.organizationsRepo.GetInvitationByID(s.ctx, invitation.ID)
			s.Require().NoError(err)
			s.Equal(organizationsmodel.RequestStatusPending, saved.Status)
			s.Require().NoError(call(recipient))
			s.Require().Error(call(recipient), "an invitation can only be handled once")
		})
	}
}
