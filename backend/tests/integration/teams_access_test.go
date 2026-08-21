//go:build integration
// +build integration

package integration

import (
	"context"
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
		roleResp, err := s.client.GetMyContestRoleWithResponse(s.ctx, org.Login, "priv-"+suffix, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMemberUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, roleResp.StatusCode())
		s.Equal("", roleResp.JSON200.Role)

		// Add team to contest with role "participant"
		addTeamResp, err := s.client.CreateContestTeamWithResponse(s.ctx, org.Login, "priv-"+suffix, &corev1.CreateContestTeamParams{
			TeamId: team.ID,
			Role:   ptrString("participant"),
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, addTeamResp.StatusCode())

		// List contest teams
		listTeamsResp, err := s.client.ListContestTeamsWithResponse(s.ctx, org.Login, "priv-"+suffix, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, listTeamsResp.StatusCode())
		s.Len(listTeamsResp.JSON200.Teams, 1)
		s.Equal(team.ID, listTeamsResp.JSON200.Teams[0].TeamId)

		// Now teamMemberUser inherits role "participant" in contest
		roleResp, err = s.client.GetMyContestRoleWithResponse(s.ctx, org.Login, "priv-"+suffix, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMemberUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, roleResp.StatusCode())
		s.Equal("participant", roleResp.JSON200.Role)

		// Remove team from contest
		delTeamResp, err := s.client.DeleteContestTeamWithResponse(s.ctx, org.Login, "priv-"+suffix, &corev1.DeleteContestTeamParams{
			TeamId: team.ID,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, delTeamResp.StatusCode())

		// Access revoked
		roleResp, err = s.client.GetMyContestRoleWithResponse(s.ctx, org.Login, "priv-"+suffix, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMemberUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal("", roleResp.JSON200.Role)
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
		probResp, err := s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMemberUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusForbidden, probResp.StatusCode())

		// Add team to problem with permission "read"
		addProbTeamResp, err := s.client.CreateProblemTeamWithResponse(s.ctx, problemID, &corev1.CreateProblemTeamParams{
			TeamId:     team.ID,
			Permission: ptrString("read"),
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, addProbTeamResp.StatusCode())

		// List problem teams
		listProbTeamsResp, err := s.client.ListProblemTeamsWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, listProbTeamsResp.StatusCode())
		s.Len(listProbTeamsResp.JSON200.Teams, 1)

		// Now teamMemberUser can view private problem
		probResp, err = s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMemberUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, probResp.StatusCode())

		// Delete problem team access
		delProbTeamResp, err := s.client.DeleteProblemTeamWithResponse(s.ctx, problemID, &corev1.DeleteProblemTeamParams{
			TeamId: team.ID,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", ownerUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, delProbTeamResp.StatusCode())

		// Access revoked again
		probResp, err = s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMemberUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusForbidden, probResp.StatusCode())
	})

	_ = adminUser
}

func ptrString(s string) *string {
	return &s
}
