//go:build integration
// +build integration

package integration

import (
	"net/http"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestTeamBasedAccess() {
	suffix := uuid.NewString()[:8]

	ownerUser := s.createUser("team_owner_"+suffix, models.UserRoleUser)
	teamMemberUser := s.createUser("team_member_"+suffix, models.UserRoleUser)
	adminUser := s.createUser("team_admin_"+suffix, models.UserRoleAdmin)

	org := s.createOrganization("team-org-"+suffix, "Team Test Org", ownerUser.Id)

	// Add teamMemberUser to org
	err := s.organizationsRepo.AddMember(s.ctx, org.ID, teamMemberUser.Id, models.OrgRoleMember)
	s.Require().NoError(err)

	// Create Team
	team, err := s.teamsRepo.CreateTeam(s.ctx, &models.CreateTeamInput{
		OrganizationID: org.ID,
		Name:           "Test Dev Team " + suffix,
		Slug:           "dev-team-" + suffix,
		Description:    "Dev Team for testing",
		Privacy:        models.TeamPrivacyClosed,
	})
	s.Require().NoError(err)

	// Add teamMemberUser to team
	err = s.teamsRepo.AddTeamMember(s.ctx, team.ID, teamMemberUser.Id, models.TeamRoleMember)
	s.Require().NoError(err)

	// 1. Contest Team Access Test
	s.Run("Contest Team Access Flow", func() {
		contestID := uuid.New()
		ownerID := ownerUser.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "Private Contest " + suffix,
			Login:          "priv-" + suffix,
			Description:    "Private contest description",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		// Before adding team, teamMemberUser has empty role in private contest
		roleResp, err := s.client.GetMyContestRole(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetMyContestRoleParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(roleResp)
		s.Equal("", roleResp.Role)

		// Add team to contest with role "participant"
		err = s.client.CreateContestTeam(withTestUser(s.ctx, ownerUser.Id), corev1.CreateContestTeamParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
			TeamID:       team.ID,
			Role:         corev1.NewOptString("participant"),
		})
		s.Require().NoError(err)

		// List contest teams
		listTeamsResp, err := s.client.ListContestTeams(withTestUser(s.ctx, ownerUser.Id), corev1.ListContestTeamsParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(listTeamsResp)
		s.Len(listTeamsResp.Teams, 1)
		s.Equal(team.ID, listTeamsResp.Teams[0].TeamID)

		// Now teamMemberUser inherits role "participant" in contest
		roleResp, err = s.client.GetMyContestRole(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetMyContestRoleParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(roleResp)
		s.Equal("participant", roleResp.Role)

		// Remove team from contest
		err = s.client.DeleteContestTeam(withTestUser(s.ctx, ownerUser.Id), corev1.DeleteContestTeamParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
			TeamID:       team.ID,
		})
		s.Require().NoError(err)

		// Access revoked
		roleResp, err = s.client.GetMyContestRole(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetMyContestRoleParams{
			OrgLogin:     org.Login,
			ContestLogin: "priv-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(roleResp)
		s.Equal("", roleResp.Role)
	})

	// 2. Problem Team Access Test
	s.Run("Problem Team Access Flow", func() {
		problemID := uuid.New()
		ownerID := ownerUser.Id

		err := s.problemsRepo.CreateProblem(s.ctx, &models.CreateProblemParams{
			ID:             problemID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ProblemVisibilityPrivate,
			Title:          "Private Problem " + suffix,
			ShortName:      "prob-" + suffix,
		})
		s.Require().NoError(err)

		// Before adding team, teamMemberUser cannot get private problem
		_, err = s.client.GetProblem(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))

		// Add team to problem with permission "read"
		err = s.client.CreateProblemTeam(withTestUser(s.ctx, ownerUser.Id), corev1.CreateProblemTeamParams{
			ID:         problemID,
			TeamID:     team.ID,
			Permission: corev1.NewOptString("read"),
		})
		s.Require().NoError(err)

		// List problem teams
		listProbTeamsResp, err := s.client.ListProblemTeams(withTestUser(s.ctx, ownerUser.Id), corev1.ListProblemTeamsParams{
			ID: problemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(listProbTeamsResp)
		s.Len(listProbTeamsResp.Teams, 1)

		// Now teamMemberUser can view private problem
		probResp, err := s.client.GetProblem(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(probResp)

		// Delete problem team access
		err = s.client.DeleteProblemTeam(withTestUser(s.ctx, ownerUser.Id), corev1.DeleteProblemTeamParams{
			ID:     problemID,
			TeamID: team.ID,
		})
		s.Require().NoError(err)

		// Access revoked again
		_, err = s.client.GetProblem(withTestUser(s.ctx, teamMemberUser.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))
	})

	_ = adminUser
}
