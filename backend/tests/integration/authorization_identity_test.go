//go:build integration

package integration

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/organizations"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
)

func (s *IntegrationTestSuite) TestMembershipRequestReadersRejectSpoofedIdentity() {
	_, contestID, _, ownerID := s.newJobSubmission()
	owner, err := s.usersRepo.GetUserById(s.ctx, ownerID)
	s.Require().NoError(err)
	contest, err := s.contestsRepo.GetContest(s.ctx, contestID)
	s.Require().NoError(err)
	target := s.createUser("request_reader_target", accessmodel.UserRoleUser)
	outsider := s.createUser("request_reader_outsider", accessmodel.UserRoleUser)
	invitation, err := s.organizationsRepo.CreateInvitation(s.ctx, &organizationsmodel.CreateOrganizationInvitationInput{OrganizationID: contest.OrganizationID, UserID: target.Id, InviterID: ownerID, Role: organizationsmodel.OrgRoleMember})
	s.Require().NoError(err)
	orgRequest, err := s.organizationsRepo.CreateJoinRequest(s.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{OrganizationID: contest.OrganizationID, UserID: target.Id})
	s.Require().NoError(err)
	contestRequest, err := s.contestsRepo.CreateContestJoinRequest(s.ctx, &contestsmodel.CreateContestJoinRequestInput{ContestID: contestID, UserID: target.Id})
	s.Require().NoError(err)
	permissions := access.NewPermissionsUseCase(s.contestsRepo, s.usersRepo, s.problemsRepo, s.teamsRepo, s.organizationsRepo)
	orgs := organizations.NewOrganizationsUseCase(s.organizationsRepo, s.usersRepo, permissions, pgstore.NewOrganizationTransactions(s.dbPool), nil)
	contestUC := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), permissions)
	for _, test := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"guest", s.ctx, pkg.ErrUnauthenticated},
		{"outsider", accessmodel.WithUser(s.ctx, outsider), pkg.NoPermission},
	} {
		s.Run(test.name, func() {
			_, err := orgs.ListInvitations(test.ctx, contest.OrganizationLogin, ownerID)
			s.ErrorIs(err, test.want)
			_, err = orgs.ListJoinRequests(test.ctx, contest.OrganizationLogin, ownerID)
			s.ErrorIs(err, test.want)
			_, err = orgs.GetMyPendingJoinRequest(test.ctx, contest.OrganizationLogin, target.Id)
			s.ErrorIs(err, test.want)
			_, err = contestUC.ListJoinRequests(test.ctx, contest.OrganizationLogin, contest.Login, ownerID)
			s.ErrorIs(err, test.want)
			_, err = contestUC.GetMyPendingJoinRequest(test.ctx, contest.OrganizationLogin, contest.Login, target.Id)
			s.ErrorIs(err, test.want)
		})
	}
	ownerCtx, targetCtx := accessmodel.WithUser(s.ctx, owner), accessmodel.WithUser(s.ctx, target)
	invitations, err := orgs.ListInvitations(ownerCtx, contest.OrganizationLogin, ownerID)
	s.Require().NoError(err)
	s.Require().Len(invitations, 1)
	s.Equal(invitation.ID, invitations[0].ID)
	orgRequests, err := orgs.ListJoinRequests(ownerCtx, contest.OrganizationLogin, ownerID)
	s.Require().NoError(err)
	s.Require().Len(orgRequests, 1)
	s.Equal(orgRequest.ID, orgRequests[0].ID)
	myOrgRequest, err := orgs.GetMyPendingJoinRequest(targetCtx, contest.OrganizationLogin, target.Id)
	s.Require().NoError(err)
	s.Require().NotNil(myOrgRequest)
	s.Equal(orgRequest.ID, myOrgRequest.ID)
	contestRequests, err := contestUC.ListJoinRequests(ownerCtx, contest.OrganizationLogin, contest.Login, ownerID)
	s.Require().NoError(err)
	s.Require().Len(contestRequests, 1)
	s.Equal(contestRequest.ID, contestRequests[0].ID)
	myContestRequest, err := contestUC.GetMyPendingJoinRequest(targetCtx, contest.OrganizationLogin, contest.Login, target.Id)
	s.Require().NoError(err)
	s.Require().NotNil(myContestRequest)
	s.Equal(contestRequest.ID, myContestRequest.ID)
}
