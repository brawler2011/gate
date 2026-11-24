//go:build integration

package integration

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/judging"
	"github.com/google/uuid"
)

// Contest submissions are judged on pretests until the end; system testing then rejudges them on all tests once.
func (s *IntegrationTestSuite) TestSystemTestingAfterPretests() {
	admin := s.createUser("admin_pretests_"+uuid.NewString()[:8], accessmodel.UserRoleAdmin)
	participant := s.createUser("participant_pretests_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("pretests-"+uuid.NewString()[:8], "Pretests", admin.Id)
	problem, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title: "Pretests", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Title: "Pretests",
		Login: "pretests", Visibility: contestsmodel.ContestVisibilityPublic}, admin.Id))
	path := corev1.UpdateContestParams{ContestID: contestID}
	err = s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{Pretests: corev1.NewOptBool(true)}, path)
	s.Equal(http.StatusBadRequest, s.getStatusCode(err), "pretests need an end time")
	setEnd := func(end time.Time) {
		start := time.Now().Add(-2 * time.Hour)
		s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID,
			StartTime: &start, StartTimeSet: true, EndTime: &end, EndTimeSet: true}))
	}
	setEnd(time.Now().Add(time.Hour))
	s.Require().NoError(s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{Pretests: corev1.NewOptBool(true)}, path))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{
		ContestId: contestID, ProblemId: problem.ID, PackageId: s.createDummyProblemPackage(problem.ID, org.ID)}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID, UserId: participant.Id, Role: accessmodel.ContestParticipant}))
	created, err := s.client.CreateSubmission(withTestUser(s.ctx, participant.Id), &corev1.CreateSubmissionRequestModel{Submission: "print(1)"},
		corev1.CreateSubmissionParams{ProblemID: problem.ID, ContestID: contestID, Language: "python3"})
	s.Require().NoError(err)
	state := func() (pretests bool, revision int64, jobs int) {
		s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT s.pretests, s.revision, (SELECT count(*) FROM judge_jobs j WHERE j.submission_id = s.id AND j.revision = s.revision)
			FROM submissions s WHERE s.id = $1`, created.ID).Scan(&pretests, &revision, &jobs))
		return
	}
	pretests, before, jobs := state()
	s.True(pretests)
	s.Equal(1, jobs)

	transactions := pgstore.NewJudgeTransactions(s.dbPool)
	count, err := judging.SystemTest(s.ctx, transactions)
	s.Require().NoError(err)
	s.Zero(count, "nothing is system tested while the contest runs")

	setEnd(time.Now().Add(-time.Minute))
	count, err = judging.SystemTest(s.ctx, transactions)
	s.Require().NoError(err)
	s.Equal(1, count)
	pretests, after, jobs := state()
	s.False(pretests, "the rejudge runs on all tests")
	s.Greater(after, before)
	s.Equal(1, jobs)
	count, err = judging.SystemTest(s.ctx, transactions)
	s.Require().NoError(err)
	s.Zero(count)
}
