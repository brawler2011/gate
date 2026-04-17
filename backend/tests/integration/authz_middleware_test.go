//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *IntegrationTestSuite) TestAuthorizationMiddleware() {
	user := s.createUser("authz_user", models.UserRoleUser)
	admin := s.createUser("authz_admin", models.UserRoleAdmin)
	target := s.createUser("authz_target", models.UserRoleUser)

	s.Run("Public endpoint without auth", func() {
		resp, err := s.client.ListPublicContestsWithResponse(s.ctx, &corev1.ListPublicContestsParams{
			Page:     1,
			PageSize: 10,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
	})

	s.Run("Protected endpoint without auth", func() {
		resp, err := s.client.GetMeWithResponse(s.ctx)
		s.Require().NoError(err)
		s.Equal(http.StatusUnauthorized, resp.StatusCode())
	})

	s.Run("Admin endpoint with user role", func() {
		resp, err := s.client.ListAdminContestsWithResponse(s.ctx, &corev1.ListAdminContestsParams{
			Page:     1,
			PageSize: 10,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", user.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusForbidden, resp.StatusCode())
	})

	s.Run("Admin endpoint with admin role", func() {
		resp, err := s.client.ListAdminContestsWithResponse(s.ctx, &corev1.ListAdminContestsParams{
			Page:     1,
			PageSize: 10,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
	})

	s.Run("DeleteAvatar requires authentication", func() {
		resp, err := s.client.DeleteAvatarWithResponse(s.ctx, target.Id)
		s.Require().NoError(err)
		s.Equal(http.StatusUnauthorized, resp.StatusCode())
	})

	s.Run("UploadAvatar custom check self/admin", func() {
		suffix := uuid.NewString()[:8]
		targetUser := s.createUser("authz_avatar_target_"+suffix, models.UserRoleUser)
		otherUser := s.createUser("authz_avatar_other_"+suffix, models.UserRoleUser)
		adminUser := s.createUser("authz_avatar_admin_"+suffix, models.UserRoleAdmin)

		newEmptyMultipartBody := func() (string, []byte) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			s.Require().NoError(writer.Close())
			return writer.FormDataContentType(), body.Bytes()
		}

		s.Run("Non-owner and non-admin denied", func() {
			contentType, body := newEmptyMultipartBody()
			resp, err := s.client.UploadAvatarWithBodyWithResponse(s.ctx, targetUser.Id, contentType, bytes.NewReader(body), func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", otherUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Self request passes middleware", func() {
			contentType, body := newEmptyMultipartBody()
			resp, err := s.client.UploadAvatarWithBodyWithResponse(s.ctx, targetUser.Id, contentType, bytes.NewReader(body), func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", targetUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			// Empty multipart body is rejected by handler, but middleware check must pass.
			s.Equal(http.StatusBadRequest, resp.StatusCode())
		})

		s.Run("Admin request passes middleware", func() {
			contentType, body := newEmptyMultipartBody()
			resp, err := s.client.UploadAvatarWithBodyWithResponse(s.ctx, targetUser.Id, contentType, bytes.NewReader(body), func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", adminUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			// Empty multipart body is rejected by handler, but middleware check must pass.
			s.Equal(http.StatusBadRequest, resp.StatusCode())
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
			ShortName:      "lus-allowed-" + suffix,
			Description:    "contest for allowed own submissions",
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
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
			ShortName:      "lus-denied-" + suffix,
			Description:    "contest for denied own submissions",
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: deniedContestID,
			UserId:    requestUser.Id,
			Role:      models.ContestRoleParticipant,
		})
		s.Require().NoError(err)

		s.Run("User cannot list another user submissions", func() {
			resp, err := s.client.ListUserSubmissionsWithResponse(s.ctx, anotherUser.Id, &corev1.ListUserSubmissionsParams{
				Page:     1,
				PageSize: 10,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", requestUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Admin can list another user submissions", func() {
			resp, err := s.client.ListUserSubmissionsWithResponse(s.ctx, anotherUser.Id, &corev1.ListUserSubmissionsParams{
				Page:     1,
				PageSize: 10,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", adminUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusOK, resp.StatusCode())
		})

		s.Run("User can list own submissions when contest policy allows", func() {
			contestID := openapi_types.UUID(allowedContestID)
			resp, err := s.client.ListUserSubmissionsWithResponse(s.ctx, requestUser.Id, &corev1.ListUserSubmissionsParams{
				Page:      1,
				PageSize:  10,
				ContestId: &contestID,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", requestUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusOK, resp.StatusCode())
		})

		s.Run("User cannot list own submissions when persisted mask denies", func() {
			_, err := s.dbPool.Exec(
				s.ctx,
				"UPDATE contest_members SET permissions_mask = $3 WHERE contest_id = $1 AND user_id = $2",
				deniedContestID,
				requestUser.Id,
				int64(models.ContestPermissionGetContest),
			)
			s.Require().NoError(err)

			roleResp, err := s.client.GetMyContestRoleWithResponse(s.ctx, deniedContestID, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", requestUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusOK, roleResp.StatusCode())
			s.Require().NotNil(roleResp.JSON200)
			s.Require().NotNil(roleResp.JSON200.PermissionsMask)
			s.Equal(int64(models.ContestPermissionGetContest), *roleResp.JSON200.PermissionsMask)

			contestID := openapi_types.UUID(deniedContestID)
			resp, err := s.client.ListUserSubmissionsWithResponse(s.ctx, requestUser.Id, &corev1.ListUserSubmissionsParams{
				Page:      1,
				PageSize:  10,
				ContestId: &contestID,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", requestUser.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
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
			ShortName:      "lcs-contest-" + suffix,
			Description:    "contest for list contest submissions custom checks",
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
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
			resp, err := s.client.ListContestSubmissionsWithResponse(s.ctx, contestID, &corev1.ListContestSubmissionsParams{
				Page:     1,
				PageSize: 10,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", participant.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Participant own submissions branch passes middleware", func() {
			selfID := openapi_types.UUID(participant.Id)
			resp, err := s.client.ListContestSubmissionsWithResponse(s.ctx, contestID, &corev1.ListContestSubmissionsParams{
				Page:     0,
				PageSize: 10,
				UserId:   &selfID,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", participant.Id.String())
				return nil
			})
			s.Require().NoError(err)
			// page=0 forces repository error before sequence lookup; middleware must allow this branch.
			s.Equal(http.StatusInternalServerError, resp.StatusCode())
		})

		s.Run("Participant cannot list other user submissions", func() {
			otherID := openapi_types.UUID(otherUser.Id)
			resp, err := s.client.ListContestSubmissionsWithResponse(s.ctx, contestID, &corev1.ListContestSubmissionsParams{
				Page:     1,
				PageSize: 10,
				UserId:   &otherID,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", participant.Id.String())
				return nil
			})
			s.Require().NoError(err)
			s.Equal(http.StatusForbidden, resp.StatusCode())
		})

		s.Run("Moderator all submissions branch passes middleware", func() {
			resp, err := s.client.ListContestSubmissionsWithResponse(s.ctx, contestID, &corev1.ListContestSubmissionsParams{
				Page:     0,
				PageSize: 10,
			}, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("X-Test-User-ID", moderator.Id.String())
				return nil
			})
			s.Require().NoError(err)
			// page=0 forces repository error before sequence lookup; middleware must allow this branch.
			s.Equal(http.StatusInternalServerError, resp.StatusCode())
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
			ShortName:      "team-contest-" + suffix,
			Description:    "private contest for authz",
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
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

		resp, err := s.client.GetContestWithResponse(s.ctx, contestID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMember.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
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
			ShortName:      "mixed-role-" + suffix,
			Description:    "contest for mixed direct+team role resolution",
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
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

		var directMask int64
		err = s.dbPool.QueryRow(
			s.ctx,
			"SELECT permissions_mask FROM contest_members WHERE contest_id = $1 AND user_id = $2",
			contestID,
			mixedUser.Id,
		).Scan(&directMask)
		s.Require().NoError(err)
		s.Equal(int64(models.ContestPermissionMaskParticipantDefault), directMask)

		var teamMask int64
		err = s.dbPool.QueryRow(
			s.ctx,
			"SELECT permissions_mask FROM contest_teams WHERE contest_id = $1 AND team_id = $2",
			contestID,
			teamID,
		).Scan(&teamMask)
		s.Require().NoError(err)
		s.Equal(int64(models.ContestPermissionMaskModeratorDefault), teamMask)

		roleResp, err := s.client.GetMyContestRoleWithResponse(s.ctx, contestID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", mixedUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, roleResp.StatusCode())
		s.Require().NotNil(roleResp.JSON200)
		s.Equal(string(models.ContestRoleModerator), roleResp.JSON200.Role)
		s.Require().NotNil(roleResp.JSON200.PermissionsMask)
		s.Equal(int64(models.ContestPermissionMaskModeratorDefault), *roleResp.JSON200.PermissionsMask)

		otherUserID := openapi_types.UUID(otherUser.Id)
		resp, err := s.client.ListContestSubmissionsWithResponse(s.ctx, contestID, &corev1.ListContestSubmissionsParams{
			Page:     0,
			PageSize: 10,
			UserId:   &otherUserID,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", mixedUser.Id.String())
			return nil
		})
		s.Require().NoError(err)
		// page=0 forces repository error before sequence lookup; middleware must allow this branch.
		s.Equal(http.StatusInternalServerError, resp.StatusCode())
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

		resp, err := s.client.GetProblemWithResponse(s.ctx, problemID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", teamMember.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
	})

	s.Run("Contest persisted mask denies participant", func() {
		suffix := uuid.NewString()[:8]
		owner := s.createUser("authz_policy_owner_"+suffix, models.UserRoleUser)
		participant := s.createUser("authz_policy_participant_"+suffix, models.UserRoleUser)

		org := s.createOrganization("authz-policy-org-"+suffix, "Authz Policy Org", owner.Id)
		contestID := uuid.New()
		ownerID := owner.Id

		err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
			ID:             contestID,
			OrganizationID: org.ID,
			OwnerID:        &ownerID,
			Visibility:     models.ContestVisibilityPrivate,
			Title:          "Policy Contest",
			ShortName:      "policy-contest-" + suffix,
			Description:    "contest with explicit policy",
			Settings:       map[string]interface{}{},
			AccessPolicy:   models.DefaultContestAccessPolicy(),
		})
		s.Require().NoError(err)

		err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
			ContestId: contestID,
			UserId:    participant.Id,
			Role:      models.ContestRoleParticipant,
		})
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(
			s.ctx,
			"UPDATE contest_members SET permissions_mask = $3 WHERE contest_id = $1 AND user_id = $2",
			contestID,
			participant.Id,
			int64(0),
		)
		s.Require().NoError(err)

		resp, err := s.client.GetContestWithResponse(s.ctx, contestID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", participant.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusForbidden, resp.StatusCode())
	})
}
