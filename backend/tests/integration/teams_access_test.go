//go:build integration
// +build integration

package integration

import (
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestTeamBasedAccess() {
	suffix := uuid.NewString()[:8]

	ownerUser := s.createUser("team_owner_"+suffix, accessmodel.UserRoleUser)
	teamMemberUser := s.createUser("team_member_"+suffix, accessmodel.UserRoleUser)
	adminUser := s.createUser("team_admin_"+suffix, accessmodel.UserRoleAdmin)

	org := s.createOrganization("team-org-"+suffix, "Team Test Org", ownerUser.Id)

	err := s.organizationsRepo.AddMember(s.ctx, org.ID, teamMemberUser.Id, organizationsmodel.OrgRoleMember)
	s.Require().NoError(err)

	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{
		OrganizationID: org.ID,
		Name:           "Test Dev Team " + suffix,
		Slug:           "dev-team-" + suffix,
		Description:    "Dev Team for testing",
		Privacy:        organizationsmodel.TeamPrivacyClosed,
	})
	s.Require().NoError(err)

	err = s.teamsRepo.AddTeamMember(s.ctx, team.ID, teamMemberUser.Id, organizationsmodel.TeamRoleMember)
	s.Require().NoError(err)

	s.Run("Contest Team Access Flow", func() {
		contestID := uuid.New()
		ownerID := ownerUser.Id

		err := s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     contestsmodel.ContestVisibilityPrivate,
			Title:          "Private Contest " + suffix,
			Login:          "priv-" + suffix,
			Description:    "Private contest description",
		})
		s.Require().NoError(err)

		// Team membership alone grants no private contest role.
		roleResp, err := s.client.GetMyContestRole(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetMyContestRoleParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(roleResp)
		s.Equal("", roleResp.Role)

		err = s.client.CreateContestTeam(withTestUser(s.ctx, ownerUser.Id), corev1.CreateContestTeamParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
			TeamID:       team.ID,
			Role:         corev1.NewOptCreateContestTeamRole(corev1.CreateContestTeamRoleParticipant),
		})
		s.Require().NoError(err)

		listTeamsResp, err := s.client.ListContestTeams(withTestUser(s.ctx, ownerUser.Id), corev1.ListContestTeamsParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(listTeamsResp)
		s.Len(listTeamsResp.Teams, 1)
		s.Equal(team.ID, listTeamsResp.Teams[0].TeamID)

		// A linked team grants its members the contest role.
		roleResp, err = s.client.GetMyContestRole(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetMyContestRoleParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(roleResp)
		s.Equal("participant", roleResp.Role)

		err = s.client.DeleteContestTeam(withTestUser(s.ctx, ownerUser.Id), corev1.DeleteContestTeamParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
			TeamID:       team.ID,
		})
		s.Require().NoError(err)

		roleResp, err = s.client.GetMyContestRole(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetMyContestRoleParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(roleResp)
		s.Equal("", roleResp.Role)
	})

	s.Run("Problem Team Access Flow", func() {
		problemID := uuid.New()
		ownerID := ownerUser.Id

		err := s.problemsRepo.CreateProblem(s.ctx, &problemsmodel.CreateProblemParams{
			ID:             problemID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     problemsmodel.ProblemVisibilityPrivate,
			Title:          "Private Problem " + suffix,
			ShortName:      "prob-" + suffix,
		})
		s.Require().NoError(err)

		// Team membership alone grants no private problem access.
		_, err = s.client.GetProblem(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusNotFound, s.getStatusCode(err))

		err = s.client.CreateProblemTeam(withTestUser(s.ctx, ownerUser.Id), corev1.CreateProblemTeamParams{
			ID:         problemID,
			TeamID:     team.ID,
			Permission: corev1.NewOptString("read"),
		})
		s.Require().NoError(err)

		listProbTeamsResp, err := s.client.ListProblemTeams(withTestUser(s.ctx, ownerUser.Id), corev1.ListProblemTeamsParams{
			ID: problemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(listProbTeamsResp)
		s.Len(listProbTeamsResp.Teams, 1)

		// A linked team grants its members private problem access.
		probResp, err := s.client.GetProblem(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(probResp)

		err = s.client.DeleteProblemTeam(withTestUser(s.ctx, ownerUser.Id), corev1.DeleteProblemTeamParams{
			ID:     problemID,
			TeamID: team.ID,
		})
		s.Require().NoError(err)

		_, err = s.client.GetProblem(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusNotFound, s.getStatusCode(err))
	})

	_ = adminUser
}
