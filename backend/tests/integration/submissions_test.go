//go:build integration
// +build integration

package integration

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/access"
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
	probResp, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(),
		Title:          problemTitle,
		OrganizationID: corev1.NewOptUUID(problemOrg.ID),
		TemplateID:     "builtin:a-plus-b",
	})
	s.Require().NoError(err)
	s.Require().NotNil(probResp)
	problemID := probResp.ID

	packageID := s.createDummyProblemPackage(problemID, problemOrg.ID)

	contestID := uuid.New()
	err = s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: contestOrg.ID,
		OwnerID:        &admin.Id,
		Title:          "Submission Contest",
		Login:          "submission-contest",
		Description:    "A test contest for submissions",
		Visibility:     contestsmodel.ContestVisibilityPublic,
	})
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
		Role:      contestsmodel.ContestRoleParticipant,
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
		s.checkDiagnosticDelivery(submissionID, admin.Id, contestOrg.Login)
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
	uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), access.NewMetadataPermissions(access.NewPermissionsUseCase(s.contestsRepo, s.usersRepo, s.problemsRepo, s.teamsRepo, s.organizationsRepo), pgstore.NewPolicyFactsRepo(s.dbPool)))
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
	var jobs, events int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM judge_jobs WHERE submission_id=$1`, sub).Scan(&jobs))
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submission.created'`, sub).Scan(&events))
	s.Equal(1, jobs)
	s.Equal(1, events)
}
