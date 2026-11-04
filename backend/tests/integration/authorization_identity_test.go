//go:build integration

package integration

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/identity"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/internal/organizations"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
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

func (s *IntegrationTestSuite) TestCommandsRejectNewlyExpiredActorOutsideHTTP() {
	_, contestID, problemID, ownerID := s.newJobSubmission()
	owner, err := s.usersRepo.GetUserById(s.ctx, ownerID)
	s.Require().NoError(err)
	oldCtx := accessmodel.WithUser(s.ctx, owner)
	contest, err := s.contestsRepo.GetContest(s.ctx, contestID)
	s.Require().NoError(err)
	org, err := s.organizationsRepo.GetOrganizationByID(s.ctx, contest.OrganizationID)
	s.Require().NoError(err)
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Original", Slug: "expiry-team", Privacy: organizationsmodel.TeamPrivacyClosed})
	s.Require().NoError(err)
	beforeProblem, err := s.problemsRepo.GetProblemById(s.ctx, problemID)
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET expires_at=now()-interval '1 hour' WHERE id=$1`, ownerID)
	s.Require().NoError(err)
	s.False(owner.IsExpired(), "the same context retains its previously authenticated actor")
	name, visibility := "denied_update", "public"
	contestUC := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
	orgUC := organizations.NewOrganizationsUseCase(s.organizationsRepo, s.usersRepo, nil, pgstore.NewOrganizationTransactions(s.dbPool), nil)
	teamUC := organizations.NewTeamsUseCase(s.teamsRepo, nil, pgstore.NewTeamTransactions(s.dbPool))
	problemUC := problems.NewProblemsUseCase(s.problemsRepo, nil, pgstore.NewProblemTransactions(s.dbPool), nil, s.organizationsRepo)
	usersUC := identity.NewUsersUseCase(s.usersRepo, pgstore.NewIdentityTransactions(s.dbPool), nil)
	for _, test := range []struct {
		name    string
		command func() error
	}{
		{"contest", func() error {
			return contestUC.UpdateContest(oldCtx, contestsmodel.ContestUpdateInput{ID: contestID, Title: &name})
		}},
		{"organization", func() error {
			return orgUC.UpdateOrganization(oldCtx, org.ID, ownerID, &organizationsmodel.UpdateOrganizationInput{Name: &name})
		}},
		{"team", func() error {
			return teamUC.UpdateTeam(oldCtx, team.ID, ownerID, &organizationsmodel.UpdateTeamInput{Name: &name})
		}},
		{"problem", func() error {
			return problemUC.UpdateProblem(oldCtx, problemID, &problemsmodel.ProblemUpdate{Visibility: &visibility})
		}},
		{"account", func() error {
			return usersUC.UpdateUser(oldCtx, identitymodel.UpdateUserInput{Id: ownerID, Username: &name})
		}},
	} {
		s.Run(test.name, func() { s.ErrorIs(test.command(), pkg.ErrUnauthenticated) })
	}
	savedContest, err := s.contestsRepo.GetContest(s.ctx, contestID)
	s.Require().NoError(err)
	s.Equal(contest.Title, savedContest.Title)
	savedOrg, err := s.organizationsRepo.GetOrganizationByID(s.ctx, org.ID)
	s.Require().NoError(err)
	s.Equal(org.Name, savedOrg.Name)
	savedTeam, err := s.teamsRepo.GetTeamByID(s.ctx, team.ID)
	s.Require().NoError(err)
	s.Equal(team.Name, savedTeam.Name)
	savedProblem, err := s.problemsRepo.GetProblemById(s.ctx, problemID)
	s.Require().NoError(err)
	s.Equal(beforeProblem.Visibility, savedProblem.Visibility)
	savedOwner, err := s.usersRepo.GetUserById(s.ctx, ownerID)
	s.Require().NoError(err)
	s.Equal(owner.Username, savedOwner.Username)
}

func (s *IntegrationTestSuite) TestAuthorizationHoldsAccountExpiryUntilCommit() {
	_, contestID, _, ownerID := s.newJobSubmission()
	checked, release := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- pgstore.NewContestCommandTransactions(s.dbPool).Within(s.ctx, func(tx contests.CommandTxStore) error {
			allowed, err := tx.HasContestPermission(s.ctx, contestID, ownerID, accessmodel.ActionManageContest)
			if err != nil {
				return err
			}
			if !allowed {
				return pkg.NoPermission
			}
			close(checked)
			<-release
			return nil
		})
	}()
	select {
	case <-checked:
	case err := <-finished:
		s.Require().NoError(err)
		s.FailNow("authorization did not run")
	case <-time.After(5 * time.Second):
		close(release)
		s.FailNow("authorization timed out")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 200*time.Millisecond)
	_, err := s.dbPool.Exec(ctx, `UPDATE users SET expires_at=now()-interval '1 hour' WHERE id=$1`, ownerID)
	cancel()
	close(release)
	s.Require().NoError(<-finished)
	s.Require().Error(err, "expiry revocation must wait for the authorized transaction")
	_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET expires_at=now()-interval '1 hour' WHERE id=$1`, ownerID)
	s.Require().NoError(err)
	err = pgstore.NewContestCommandTransactions(s.dbPool).Within(s.ctx, func(tx contests.CommandTxStore) error {
		allowed, err := tx.HasContestPermission(s.ctx, contestID, ownerID, accessmodel.ActionManageContest)
		s.False(allowed)
		return err
	})
	s.ErrorIs(err, pkg.ErrUnauthenticated, "the next transaction must observe the committed expiry")
}
