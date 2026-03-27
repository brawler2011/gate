//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *IntegrationTestSuite) TestSubmissions() {
	admin := s.createUser("admin_submissions", models.UserRoleAdmin)
	user := s.createUser("user_submissions", models.UserRoleUser)

	problemOrg := s.createOrganization("admin-submissions-org", "Admin Submissions Organization", admin.Id)

	// Create organization for contest
	contestOrg := s.createOrganization("contest-org", "Contest Organization", admin.Id)

	// 1. Create Problem
	problemTitle := "Submission Problem"
	problemOrganizationID := openapi_types.UUID(problemOrg.ID)
	probResp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
		Title:          problemTitle,
		OrganizationId: &problemOrganizationID,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, probResp.StatusCode())
	s.Require().NotNil(probResp.JSON200)
	problemID := probResp.JSON200.Id

	// Create a dummy problem package (required for contest_problems foreign key)
	packageID := s.createDummyProblemPackage(problemID, problemOrg.ID)

	// 2. Create Contest
	contestID := uuid.New()
	err = s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
		ID:             contestID,
		OrganizationID: contestOrg.ID,
		OwnerID:        &admin.Id,
		Titles:         map[string]string{"en": "Submission Contest"},
		ShortName:      "submission-contest",
		Description:    "A test contest for submissions",
		Visibility:     models.ContestVisibilityPublic,
		Settings:       make(map[string]interface{}),
		AccessPolicy:   make(map[string]interface{}),
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
		// Note: NATS Publish is not called directly during submission creation
		// It's done asynchronously by the outbox worker

		resp, err := s.client.CreateSubmissionWithResponse(s.ctx, &corev1.CreateSubmissionParams{
			ProblemId: problemID,
			ContestId: contestID,
			Language:  30, // Python
		}, corev1.CreateSubmissionJSONRequestBody{
			Submission: "print('hello')",
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", user.Id.String())
			return nil
		})
		s.Require().NoError(err)
		if resp.StatusCode() != http.StatusOK {
			s.T().Logf("CreateSubmission failed: %s", string(resp.Body))
		}
		s.Equal(http.StatusOK, resp.StatusCode())
		s.Require().NotNil(resp.JSON200)
		submissionID = resp.JSON200.Id
	})

	// 4. Get Submission
	s.Run("GetSubmission", func() {
		resp, err := s.client.GetSubmissionWithResponse(s.ctx, submissionID, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", user.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
		s.Equal(submissionID, resp.JSON200.Submission.Id)
	})

	// 5. List Submissions
	s.Run("ListSubmissions", func() {
		resp, err := s.client.ListSubmissionsWithResponse(s.ctx, &corev1.ListSubmissionsParams{
			Page:     1,
			PageSize: 10,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", admin.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
		s.Require().NotNil(resp.JSON200)
		s.GreaterOrEqual(len(resp.JSON200.Submissions), 1)
	})
}
