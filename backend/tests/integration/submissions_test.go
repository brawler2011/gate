//go:build integration
// +build integration

package integration

import (
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestSubmissions() {
	admin := s.createUser("admin_submissions", models.UserRoleAdmin)
	user := s.createUser("user_submissions", models.UserRoleUser)

	problemOrg := s.createOrganization("admin-submissions-org", "Admin Submissions Organization", admin.Id)

	// Create organization for contest
	contestOrg := s.createOrganization("contest-org", "Contest Organization", admin.Id)

	// 1. Create Problem
	problemTitle := "Submission Problem"
	probResp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title:          problemTitle,
		OrganizationID: corev1.NewOptUUID(problemOrg.ID),
		TemplateID:     "builtin:a-plus-b",
	})
	s.Require().NoError(err)
	s.Require().NotNil(probResp)
	problemID := probResp.ID

	// Create a dummy problem package (required for contest_problems foreign key)
	packageID := s.createDummyProblemPackage(problemID, problemOrg.ID)

	// 2. Create Contest
	contestID := uuid.New()
	err = s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
		ID:             contestID,
		OrganizationID: contestOrg.ID,
		OwnerID:        &admin.Id,
		Title:          "Submission Contest",
		Login:          "submission-contest",
		Description:    "A test contest for submissions",
		Visibility:     models.ContestVisibilityPublic,
		Settings:       make(map[string]interface{}),
	})
	s.Require().NoError(err)

	// Update contest to be active (started and not finished)
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	err = s.contestsRepo.UpdateContest(s.ctx, models.ContestUpdateParams{
		ID:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	s.Require().NoError(err)

	// Add problem to contest
	err = s.contestsRepo.CreateContestProblem(s.ctx, models.ContestProblemCreation{
		ContestId: contestID,
		ProblemId: problemID,
		PackageId: packageID,
	})
	s.Require().NoError(err)

	// Add user to contest
	err = s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{
		ContestId: contestID,
		UserId:    user.Id,
		Role:      models.ContestRoleParticipant,
	})
	s.Require().NoError(err)

	var submissionID uuid.UUID

	// 3. Create Submission
	s.Run("CreateSubmission", func() {
		resp, err := s.client.CreateSubmission(withTestUser(s.ctx, user.Id), &corev1.CreateSubmissionRequestModel{
			Submission: "print('hello')",
		}, corev1.CreateSubmissionParams{
			ProblemID:         problemID,
			OrganizationLogin: contestOrg.Login,
			ContestLogin:      "submission-contest",
			Language:          30, // Python
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		submissionID = resp.ID
	})

	// 4. Get Submission
	s.Run("GetSubmission", func() {
		resp, err := s.client.GetSubmission(withTestUser(s.ctx, user.Id), corev1.GetSubmissionParams{
			SubmissionID: submissionID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(submissionID, resp.Submission.ID)
	})

	// 5. List Submissions
	s.Run("ListSubmissions", func() {
		resp, err := s.client.ListSubmissions(withTestUser(s.ctx, admin.Id), corev1.ListSubmissionsParams{
			Page:     1,
			PageSize: 10,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.GreaterOrEqual(len(resp.Submissions), 1)
	})
}
