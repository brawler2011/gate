//go:build integration
// +build integration

package integration

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestSubmissions() {
	admin := s.createUser("admin_submissions", accessmodel.UserRoleAdmin)
	user := s.createUser("user_submissions", accessmodel.UserRoleUser)

	problemOrg := s.createOrganization("admin-submissions-org", "Admin Submissions Organization", admin.Id)

	contestOrg := s.createOrganization("contest-org", "Contest Organization", admin.Id)

	problemTitle := "Submission Problem"
	probResp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title:          problemTitle,
		OrganizationID: corev1.NewOptUUID(problemOrg.ID),
		TemplateID:     "builtin:a-plus-b",
	})
	s.Require().NoError(err)
	s.Require().NotNil(probResp)
	problemID := probResp.ID

	packageID := s.createDummyProblemPackage(problemID, problemOrg.ID)

	contestID := uuid.New()
	err = s.createContest(contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: contestOrg.ID,
		Title:          "Submission Contest",
		Login:          "submission-contest",
		Description:    "A test contest for submissions",
		Visibility:     contestsmodel.ContestVisibilityPublic,
	}, admin.Id)
	s.Require().NoError(err)

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	err = s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{
		ID:           contestID,
		StartTime:    &startTime,
		StartTimeSet: true,
		EndTime:      &endTime,
		EndTimeSet:   true,
	})
	s.Require().NoError(err)

	err = s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{
		ContestId: contestID,
		ProblemId: problemID,
		PackageId: packageID,
	})
	s.Require().NoError(err)

	err = s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID,
		UserId:    user.Id,
		Role:      accessmodel.ContestParticipant,
	})
	s.Require().NoError(err)

	var submissionID uuid.UUID

	s.Run("CreateSubmission", func() {
		resp, err := s.client.CreateSubmission(withTestUser(s.ctx, user.Id), &corev1.CreateSubmissionRequestModel{
			Submission: "print('hello')",
		}, corev1.CreateSubmissionParams{
			ProblemID:         problemID,
			OrganizationLogin: contestOrg.Login,
			ContestLogin:      "submission-contest",
			Language:          "python3",
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		submissionID = resp.ID
		// Rapid consecutive submissions are valid; quotas impose no inter-submit delay.
		for range 2 {
			_, err := s.client.CreateSubmission(withTestUser(s.ctx, user.Id), &corev1.CreateSubmissionRequestModel{
				Submission: "print('hello')",
			}, corev1.CreateSubmissionParams{
				ProblemID: problemID, OrganizationLogin: contestOrg.Login,
				ContestLogin: "submission-contest", Language: "python3",
			})
			s.Require().NoError(err)
		}
	})

	// Submissions accept only catalog configurations allowed by the contest.
	s.Run("ContestLanguageRestriction", func() {
		admin := withTestUser(s.ctx, admin.Id)
		contest := corev1.UpdateContestParams{OrgLogin: contestOrg.Login, ContestLogin: "submission-contest"}
		err := s.client.UpdateContest(admin, &corev1.UpdateContestRequestModel{Languages: []string{"cpp14"}}, contest)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
		s.Require().NoError(s.client.UpdateContest(admin, &corev1.UpdateContestRequestModel{Languages: []string{"cpp17"}}, contest))
		got, err := s.client.GetContest(admin, corev1.GetContestParams{OrgLogin: contestOrg.Login, ContestLogin: "submission-contest"})
		s.Require().NoError(err)
		s.Equal([]string{"cpp17"}, got.Contest.Languages)

		submit := func(language string) error {
			_, err := s.client.CreateSubmission(withTestUser(s.ctx, user.Id), &corev1.CreateSubmissionRequestModel{Submission: "int main() {}"}, corev1.CreateSubmissionParams{
				ProblemID: problemID, OrganizationLogin: contestOrg.Login, ContestLogin: "submission-contest", Language: language,
			})
			return err
		}
		s.Equal(http.StatusBadRequest, s.getStatusCode(submit("python3")))
		s.Equal(http.StatusBadRequest, s.getStatusCode(submit("cpp14")))
		s.Require().NoError(submit("cpp17"))

		// An empty language list allows the whole catalog.
		s.Require().NoError(s.client.UpdateContest(admin, &corev1.UpdateContestRequestModel{Languages: []string{}}, contest))
		s.Require().NoError(submit("python3"))
	})

	s.Run("GetSubmission", func() {
		resp, err := s.client.GetSubmission(withTestUser(s.ctx, user.Id), corev1.GetSubmissionParams{
			SubmissionID: submissionID,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(submissionID, resp.Submission.ID)
	})

	s.Run("ListSubmissions", func() {
		resp, err := s.client.ListSubmissions(withTestUser(s.ctx, admin.Id), corev1.ListSubmissionsParams{
			Page:     corev1.NewOptInt32(1),
			PageSize: corev1.NewOptInt32(10),
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.GreaterOrEqual(len(resp.Submissions), 1)
	})
}

func (s *IntegrationTestSuite) TestUnblockProblemQueuesOneGeneration() {
	sub, contestID, problemID, userID := s.newJobSubmission()
	actor, err := s.usersRepo.GetUserById(s.ctx, userID)
	s.Require().NoError(err)
	ctx := accessmodel.WithUser(s.ctx, actor)
	uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), pgstore.NewAccess(s.dbPool))
	s.Require().NoError(uc.BlockProblemForUser(ctx, contestID, userID, problemID, nil))
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	before, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	contest, err := s.contestsRepo.GetContest(s.ctx, contestID)
	s.Require().NoError(err)
	s.Require().NoError(s.client.UnblockProblemForUser(withTestUser(s.ctx, userID), corev1.UnblockProblemForUserParams{
		OrgLogin: contest.OrganizationLogin, ContestLogin: contest.Login, UserID: userID, ProblemID: problemID, RejudgeSubmissions: corev1.NewOptBool(true),
	}))
	after, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(before.Generation+1, after.Generation)
	s.Equal(1, s.judgeJobCount(sub))
}

// Upsolving submissions are judged and listed but never reach the standings, even when the end later moves past them.
func (s *IntegrationTestSuite) TestUpsolving() {
	admin := s.createUser("admin_upsolving", accessmodel.UserRoleAdmin)
	participant := s.createUser("participant_upsolving", accessmodel.UserRoleUser)
	outsider := s.createUser("outsider_upsolving", accessmodel.UserRoleUser)
	org := s.createOrganization("upsolving-org", "Upsolving Organization", admin.Id)
	problem, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title: "Upsolving Problem", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Title: "Upsolving",
		Login: "upsolving", Visibility: contestsmodel.ContestVisibilityPublic}, admin.Id))
	setEnd := func(end time.Time) {
		start := time.Now().Add(-2 * time.Hour)
		s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID,
			StartTime: &start, StartTimeSet: true, EndTime: &end, EndTimeSet: true}))
	}
	setEnd(time.Now().Add(-time.Hour))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{
		ContestId: contestID, ProblemId: problem.ID, PackageId: s.createDummyProblemPackage(problem.ID, org.ID)}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID, UserId: participant.Id, Role: accessmodel.ContestParticipant}))
	submit := func(user uuid.UUID) (uuid.UUID, error) {
		resp, err := s.client.CreateSubmission(withTestUser(s.ctx, user), &corev1.CreateSubmissionRequestModel{Submission: "print(1)"},
			corev1.CreateSubmissionParams{ProblemID: problem.ID, OrganizationLogin: org.Login, ContestLogin: "upsolving", Language: "python3"})
		if err != nil {
			return uuid.Nil, err
		}
		return resp.ID, nil
	}

	upsolved, err := submit(participant.Id)
	s.Require().NoError(err)
	_, err = submit(outsider.Id)
	s.Require().NoError(err, "anyone who can view a finished contest may upsolve")
	list, err := s.client.ListContestSubmissions(withTestUser(s.ctx, admin.Id), corev1.ListContestSubmissionsParams{OrgLogin: org.Login, ContestLogin: "upsolving"})
	s.Require().NoError(err)
	s.Require().Len(list.Submissions, 2)
	for _, sub := range list.Submissions {
		s.Equal(corev1.SubmissionKindUpsolving, sub.Kind)
	}

	setEnd(time.Now().Add(time.Hour))
	s.Require().NoError(s.client.BlockSubmission(withTestUser(s.ctx, admin.Id), corev1.NewOptBlockSubmissionRequestModel(corev1.BlockSubmissionRequestModel{}),
		corev1.BlockSubmissionParams{OrgLogin: org.Login, ContestLogin: "upsolving", SubmissionID: upsolved}))
	board, err := s.client.GetContestScoreboard(withTestUser(s.ctx, admin.Id), corev1.GetContestScoreboardParams{OrgLogin: org.Login, ContestLogin: "upsolving"})
	s.Require().NoError(err)
	s.Require().Len(board.Items, 1)
	for _, result := range board.Items[0].ProblemResults {
		s.False(result.Solved)
		s.Zero(result.FailedAttempts)
	}

	setEnd(time.Now().Add(-time.Hour))
	s.Require().NoError(s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{EnableUpsolving: corev1.NewOptBool(false)},
		corev1.UpdateContestParams{OrgLogin: org.Login, ContestLogin: "upsolving"}))
	_, err = submit(participant.Id)
	var problemErr *corev1.ProblemStatusCodeWithHeaders
	s.Require().ErrorAs(err, &problemErr)
	s.Equal(http.StatusConflict, problemErr.StatusCode)
	s.Equal(corev1.ErrorCodeContestEnded, problemErr.Response.Code)
}
