package integration

import (
	"context"
	"net/http"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func (s *IntegrationTestSuite) TestSubmissions() {
	admin := s.createUser("admin_submissions", models.UserRoleAdmin)
	user := s.createUser("user_submissions", models.UserRoleUser)

	// 1. Create Problem
	s.mockPandoc.EXPECT().BatchConvertLatexToHtml5(gomock.Any(), gomock.Any()).Return([]string{"html content"}, nil).AnyTimes()

	problemTitle := "Submission Problem"
	probResp, err := s.client.CreateProblemWithResponse(s.ctx, &corev1.CreateProblemParams{
		Title: problemTitle,
	}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-Test-User-ID", admin.Id.String())
		return nil
	})
	s.Require().NoError(err)
	problemID := probResp.JSON200.Id

	// 2. Create Contest
	contestID := uuid.New()
	err = s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
		Id:     contestID,
		Title:  "Submission Contest",
		UserId: admin.Id,
	})
	s.Require().NoError(err)

	// Update contest to be active (started and not finished)
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	err = s.contestsRepo.UpdateContest(s.ctx, models.ContestUpdateParams{
		Id:        contestID,
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	s.Require().NoError(err)

	// Add problem to contest
	err = s.contestsRepo.CreateContestProblem(s.ctx, models.ContestProblemCreation{
		ContestId: contestID,
		ProblemId: problemID,
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
		s.mockNats.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil).Times(1)

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
