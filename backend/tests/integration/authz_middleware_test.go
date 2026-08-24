//go:build integration
// +build integration

package integration

import (
	"bytes"
	"net/http"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
)

func (s *IntegrationTestSuite) TestAuthorizationMiddleware() {
	user := s.createUser("authz_user", models.UserRoleUser)
	admin := s.createUser("authz_admin", models.UserRoleAdmin)
	target := s.createUser("authz_target", models.UserRoleUser)

	s.Run("Public endpoint without auth", func() {
		resp, err := s.client.ListPublicContests(s.ctx, corev1.ListPublicContestsParams{
			Page:     1,
			PageSize: 10,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
	})

	s.Run("Protected endpoint without auth", func() {
		_, err := s.client.GetMe(s.ctx)
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
	})

	s.Run("Admin endpoint with user role", func() {
		_, err := s.client.ListAdminContests(withTestUser(s.ctx, user.Id), corev1.ListAdminContestsParams{
			Page:     1,
			PageSize: 10,
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))
	})

	s.Run("Admin endpoint with admin role", func() {
		resp, err := s.client.ListAdminContests(withTestUser(s.ctx, admin.Id), corev1.ListAdminContestsParams{
			Page:     1,
			PageSize: 10,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
	})

	s.Run("DeleteAvatar requires authentication", func() {
		err := s.client.DeleteAvatar(s.ctx, corev1.DeleteAvatarParams{
			Username: target.Username,
		})
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
	})

	s.Run("UploadAvatar custom check self/admin", func() {
		suffix := uuid.NewString()[:8]
		targetUser := s.createUser("authz_avatar_target_"+suffix, models.UserRoleUser)
		otherUser := s.createUser("authz_avatar_other_"+suffix, models.UserRoleUser)
		adminUser := s.createUser("authz_avatar_admin_"+suffix, models.UserRoleAdmin)

		sampleFile := ht.MultipartFile{
			Name: "avatar.png",
			File: bytes.NewReader([]byte("test")),
			Size: 4,
		}

		s.Run("Non-owner and non-admin denied", func() {
			_, err := s.client.UploadAvatar(withTestUser(s.ctx, otherUser.Id), &corev1.UploadAvatarReq{
				Avatar: corev1.NewOptMultipartFile(sampleFile),
			}, corev1.UploadAvatarParams{
				Username: targetUser.Username,
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Self request passes middleware", func() {
			// Middleware passes (not 401/403)
			_, err := s.client.UploadAvatar(withTestUser(s.ctx, targetUser.Id), &corev1.UploadAvatarReq{
				Avatar: corev1.NewOptMultipartFile(sampleFile),
			}, corev1.UploadAvatarParams{
				Username: targetUser.Username,
			})
			s.NotEqual(http.StatusForbidden, s.getStatusCode(err))
			s.NotEqual(http.StatusUnauthorized, s.getStatusCode(err))
		})

		s.Run("Admin request passes middleware", func() {
			// Middleware passes (not 401/403)
			_, err := s.client.UploadAvatar(withTestUser(s.ctx, adminUser.Id), &corev1.UploadAvatarReq{
				Avatar: corev1.NewOptMultipartFile(sampleFile),
			}, corev1.UploadAvatarParams{
				Username: targetUser.Username,
			})
			s.NotEqual(http.StatusForbidden, s.getStatusCode(err))
			s.NotEqual(http.StatusUnauthorized, s.getStatusCode(err))
		})
	})

	s.Run("ListUserSubmissions custom check", func() {
		suffix := uuid.NewString()[:8]
		contestOwner := s.createUser("authz_lus_owner_"+suffix, models.UserRoleUser)
		requestUser := s.createUser("authz_lus_user_"+suffix, models.UserRoleUser)
		anotherUser := s.createUser("authz_lus_another_"+suffix, models.UserRoleUser)
		adminUser := s.createUser("authz_lus_admin_"+suffix, models.UserRoleAdmin)

		org := s.createOrganization("authz-lus-org-"+suffix, "Authz LUS Org", contestOwner.Id)

		ownerID := contestOwner.Id
		allowedContestID := uuid.New()
		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             allowedContestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "LUS Allowed Contest",
			Login:          "lus-allowed-" + suffix,
			Description:    "contest for allowed own submissions",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: allowedContestID,
			UserId:    requestUser.Id,
			Role:      models.ContestRoleParticipant,
		})
		s.Require().NoError(err)

		deniedContestID := uuid.New()
		err = s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             deniedContestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "LUS Denied Contest",
			Login:          "lus-denied-" + suffix,
			Description:    "contest for denied own submissions",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: deniedContestID,
			UserId:    requestUser.Id,
			Role:      models.ContestRoleParticipant,
		})
		s.Require().NoError(err)

		s.Run("User cannot list another user submissions", func() {
			_, err := s.client.ListUserSubmissions(withTestUser(s.ctx, requestUser.Id), corev1.ListUserSubmissionsParams{
				Username: anotherUser.Username,
				Page:     1,
				PageSize: 10,
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Admin can list another user submissions", func() {
			resp, err := s.client.ListUserSubmissions(withTestUser(s.ctx, adminUser.Id), corev1.ListUserSubmissionsParams{
				Username: anotherUser.Username,
				Page:     1,
				PageSize: 10,
			})
			s.Require().NoError(err)
			s.Require().NotNil(resp)
		})

		s.Run("User can list own submissions when contest policy allows", func() {
			resp, err := s.client.ListUserSubmissions(withTestUser(s.ctx, requestUser.Id), corev1.ListUserSubmissionsParams{
				Username:  requestUser.Username,
				Page:      1,
				PageSize:  10,
				ContestId: corev1.NewOptUUID(allowedContestID),
			})
			s.Require().NoError(err)
			s.Require().NotNil(resp)
		})

		s.Run("User cannot list submissions when not a member of private contest", func() {
			_, err := s.client.ListUserSubmissions(withTestUser(s.ctx, anotherUser.Id), corev1.ListUserSubmissionsParams{
				Username:  anotherUser.Username,
				Page:      1,
				PageSize:  10,
				ContestId: corev1.NewOptUUID(allowedContestID),
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})
	})

	s.Run("ListContestSubmissions custom check", func() {
		suffix := uuid.NewString()[:8]
		owner := s.createUser("authz_lcs_owner_"+suffix, models.UserRoleUser)
		participant := s.createUser("authz_lcs_participant_"+suffix, models.UserRoleUser)
		moderator := s.createUser("authz_lcs_moderator_"+suffix, models.UserRoleUser)
		otherUser := s.createUser("authz_lcs_other_"+suffix, models.UserRoleUser)

		org := s.createOrganization("authz-lcs-org-"+suffix, "Authz LCS Org", owner.Id)
		contestID := uuid.New()
		ownerID := owner.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "LCS Contest",
			Login:          "lcs-contest-" + suffix,
			Description:    "contest for list contest submissions custom checks",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: contestID,
			UserId:    participant.Id,
			Role:      models.ContestRoleParticipant,
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: contestID,
			UserId:    moderator.Id,
			Role:      models.ContestRoleModerator,
		})
		s.Require().NoError(err)

		s.Run("Participant cannot list all contest submissions", func() {
			_, err := s.client.ListContestSubmissions(withTestUser(s.ctx, participant.Id), corev1.ListContestSubmissionsParams{
				OrgLogin:     org.Login,
				ContestLogin: "lcs-contest-" + suffix,
				Page:         1,
				PageSize:     10,
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Participant own submissions branch passes middleware", func() {
			resp, err := s.client.ListContestSubmissions(withTestUser(s.ctx, participant.Id), corev1.ListContestSubmissionsParams{
				OrgLogin:     org.Login,
				ContestLogin: "lcs-contest-" + suffix,
				Page:         1,
				PageSize:     10,
				UserId:       corev1.NewOptUUID(participant.Id),
			})
			s.Require().NoError(err)
			s.Require().NotNil(resp)
		})

		s.Run("Participant cannot list other user submissions", func() {
			_, err := s.client.ListContestSubmissions(withTestUser(s.ctx, participant.Id), corev1.ListContestSubmissionsParams{
				OrgLogin:     org.Login,
				ContestLogin: "lcs-contest-" + suffix,
				Page:         1,
				PageSize:     10,
				UserId:       corev1.NewOptUUID(otherUser.Id),
			})
			s.Require().Error(err)
			s.Equal(http.StatusForbidden, s.getStatusCode(err))
		})

		s.Run("Moderator all submissions branch passes middleware", func() {
			resp, err := s.client.ListContestSubmissions(withTestUser(s.ctx, moderator.Id), corev1.ListContestSubmissionsParams{
				OrgLogin:     org.Login,
				ContestLogin: "lcs-contest-" + suffix,
				Page:         1,
				PageSize:     10,
			})
			s.Require().NoError(err)
			s.Require().NotNil(resp)
		})
	})

	s.Run("Contest access through team role", func() {
		suffix := uuid.NewString()[:8]
		owner := s.createUser("authz_contest_owner_"+suffix, models.UserRoleUser)
		teamMember := s.createUser("authz_contest_member_"+suffix, models.UserRoleUser)

		org := s.createOrganization("authz-contest-org-"+suffix, "Authz Contest Org", owner.Id)
		contestID := uuid.New()
		ownerID := owner.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "Team Contest",
			Login:          "team-contest-" + suffix,
			Description:    "private contest for authz",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		teamID := uuid.New()
		_, err = s.dbPool.Exec(s.ctx,
			"INSERT INTO teams (id, organization_id, name, slug, description, privacy) VALUES ($1, $2, $3, $4, $5, $6)",
			teamID,
			org.ID,
			"Authz Contest Team",
			"authz-contest-team-"+suffix,
			"",
			string(models.TeamPrivacyClosed),
		)
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx,
			"INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)",
			teamID,
			teamMember.Id,
			string(models.TeamRoleMember),
		)
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestTeam(s.ctx, contestID, teamID, models.ContestRoleParticipant)
		s.Require().NoError(err)

		resp, err := s.client.GetContest(withTestUser(s.ctx, teamMember.Id), corev1.GetContestParams{
			OrgLogin:     org.Login,
			ContestLogin: "team-contest-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
	})

	s.Run("Higher contest role resolved from team with mask persistence", func() {
		suffix := uuid.NewString()[:8]
		owner := s.createUser("authz_mix_owner_"+suffix, models.UserRoleUser)
		mixedUser := s.createUser("authz_mix_user_"+suffix, models.UserRoleUser)
		otherUser := s.createUser("authz_mix_other_"+suffix, models.UserRoleUser)

		org := s.createOrganization("authz-mix-org-"+suffix, "Authz Mix Org", owner.Id)
		contestID := uuid.New()
		ownerID := owner.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "Mixed Role Contest",
			Login:          "mixed-role-" + suffix,
			Description:    "contest for mixed direct+team role resolution",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: contestID,
			UserId:    mixedUser.Id,
			Role:      models.ContestRoleParticipant,
		})
		s.Require().NoError(err)

		teamID := uuid.New()
		_, err = s.dbPool.Exec(s.ctx,
			"INSERT INTO teams (id, organization_id, name, slug, description, privacy) VALUES ($1, $2, $3, $4, $5, $6)",
			teamID,
			org.ID,
			"Authz Mixed Team",
			"authz-mixed-team-"+suffix,
			"",
			string(models.TeamPrivacyClosed),
		)
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx,
			"INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)",
			teamID,
			mixedUser.Id,
			string(models.TeamRoleMember),
		)
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestTeam(s.ctx, contestID, teamID, models.ContestRoleModerator)
		s.Require().NoError(err)

		roleResp, err := s.client.GetMyContestRole(withTestUser(s.ctx, mixedUser.Id), corev1.GetMyContestRoleParams{
			OrgLogin:     org.Login,
			ContestLogin: "mixed-role-" + suffix,
		})
		s.Require().NoError(err)
		s.Require().NotNil(roleResp)
		s.Equal(string(models.ContestRoleModerator), roleResp.Role)

		resp, err := s.client.ListContestSubmissions(withTestUser(s.ctx, mixedUser.Id), corev1.ListContestSubmissionsParams{
			OrgLogin:     org.Login,
			ContestLogin: "mixed-role-" + suffix,
			Page:         1,
			PageSize:     10,
			UserId:       corev1.NewOptUUID(otherUser.Id),
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
	})

	s.Run("Problem access through team permission", func() {
		suffix := uuid.NewString()[:8]
		owner := s.createUser("authz_problem_owner_"+suffix, models.UserRoleUser)
		teamMember := s.createUser("authz_problem_member_"+suffix, models.UserRoleUser)

		org := s.createOrganization("authz-problem-org-"+suffix, "Authz Problem Org", owner.Id)
		problemID := uuid.New()
		ownerID := owner.Id

		_, err := s.dbPool.Exec(s.ctx,
			"INSERT INTO problems (id, organization_id, owner_id, visibility, title, short_name) VALUES ($1, $2, $3, $4, $5, $6)",
			problemID,
			org.ID,
			ownerID,
			models.ProblemVisibilityPrivate,
			"Team Problem",
			"team-problem-"+suffix,
		)
		s.Require().NoError(err)

		teamID := uuid.New()
		_, err = s.dbPool.Exec(s.ctx,
			"INSERT INTO teams (id, organization_id, name, slug, description, privacy) VALUES ($1, $2, $3, $4, $5, $6)",
			teamID,
			org.ID,
			"Authz Problem Team",
			"authz-problem-team-"+suffix,
			"",
			string(models.TeamPrivacyClosed),
		)
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx,
			"INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)",
			teamID,
			teamMember.Id,
			string(models.TeamRoleMember),
		)
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx,
			"INSERT INTO problem_teams (problem_id, team_id, permission) VALUES ($1, $2, $3)",
			problemID,
			teamID,
			models.ProblemPermissionRead,
		)
		s.Require().NoError(err)

		resp, err := s.client.GetProblem(withTestUser(s.ctx, teamMember.Id), corev1.GetProblemParams{
			ID: problemID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
	})

	s.Run("Non-member cannot view private contest", func() {
		suffix := uuid.NewString()[:8]
		owner := s.createUser("authz_policy_owner_"+suffix, models.UserRoleUser)
		nonMember := s.createUser("authz_pol_part_"+suffix, models.UserRoleUser)

		org := s.createOrganization("authz-policy-org-"+suffix, "Authz Policy Org", owner.Id)
		contestID := uuid.New()
		ownerID := owner.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "Private Contest",
			Login:          "policy-contest-" + suffix,
			Description:    "contest with private access",
			Settings:       map[string]interface{}{},
		})
		s.Require().NoError(err)

		_, err = s.client.GetContest(withTestUser(s.ctx, nonMember.Id), corev1.GetContestParams{
			OrgLogin:     org.Login,
			ContestLogin: "policy-contest-" + suffix,
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))
	})
}
