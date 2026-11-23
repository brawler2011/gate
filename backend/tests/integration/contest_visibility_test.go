//go:build integration

package integration

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

// An olympiad with paper statements hides participants' own results until the end so a remote helper learns nothing.
func (s *IntegrationTestSuite) TestHiddenOwnResultsUntilTheEnd() {
	admin := s.createUser("admin_hidden_"+uuid.NewString()[:8], accessmodel.UserRoleAdmin)
	participant := s.createUser("participant_hidden_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("hidden-"+uuid.NewString()[:8], "Hidden Results", admin.Id)
	problem, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title: "Hidden", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Title: "Hidden",
		Login: "hidden", Visibility: contestsmodel.ContestVisibilityPublic}, admin.Id))
	setEnd := func(end time.Time) {
		start := time.Now().Add(-2 * time.Hour)
		s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID,
			StartTime: &start, StartTimeSet: true, EndTime: &end, EndTimeSet: true}))
	}
	setEnd(time.Now().Add(time.Hour))
	path := corev1.UpdateContestParams{OrgLogin: org.Login, ContestLogin: "hidden"}
	s.Require().NoError(s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{
		FeedbackDuring: corev1.NewOptFeedbackLevel(corev1.FeedbackLevelHidden), FeedbackAfter: corev1.NewOptFeedbackLevel(corev1.FeedbackLevelSamples),
		ScoreboardDuring: corev1.NewOptContestScope(corev1.ContestScopeParticipant)}, path))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{
		ContestId: contestID, ProblemId: problem.ID, PackageId: s.createDummyProblemPackage(problem.ID, org.ID)}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID, UserId: participant.Id, Role: accessmodel.ContestParticipant}))
	as := withTestUser(s.ctx, participant.Id)
	created, err := s.client.CreateSubmission(as, &corev1.CreateSubmissionRequestModel{Submission: "print(1)"},
		corev1.CreateSubmissionParams{ProblemID: problem.ID, OrganizationLogin: org.Login, ContestLogin: "hidden", Language: "python3"})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `UPDATE submissions SET state=200, test_details=$2 WHERE id=$1`, created.ID,
		`{"tests":[{"test_index":1,"verdict":"OK","sample":true},{"test_index":2,"verdict":"OK"}]}`)
	s.Require().NoError(err)
	status := func(err error) int {
		var problemErr *corev1.ProblemStatusCodeWithHeaders
		s.Require().ErrorAs(err, &problemErr)
		return problemErr.StatusCode
	}

	own, err := s.client.GetSubmission(as, corev1.GetSubmissionParams{SubmissionID: created.ID})
	s.Require().NoError(err)
	s.True(own.Submission.ResultHidden.Value)
	s.False(own.Submission.State.IsSet())
	s.False(own.Submission.Submission.IsSet(), "the code stays hidden too")
	s.False(own.Submission.TestDetails.IsSet())
	_, err = s.client.ListContestSubmissions(as, corev1.ListContestSubmissionsParams{OrgLogin: org.Login, ContestLogin: "hidden",
		UserId: corev1.NewOptUUID(participant.Id), State: corev1.NewOptInt32(200)})
	s.Equal(http.StatusForbidden, status(err), "a state filter would reveal the verdict")
	_, err = s.client.GetContestScoreboard(as, corev1.GetContestScoreboardParams{OrgLogin: org.Login, ContestLogin: "hidden"})
	s.Equal(http.StatusForbidden, status(err), "the scoreboard would reveal the verdict")

	setEnd(time.Now().Add(-time.Minute))
	own, err = s.client.GetSubmission(as, corev1.GetSubmissionParams{SubmissionID: created.ID})
	s.Require().NoError(err)
	s.False(own.Submission.ResultHidden.Value)
	s.EqualValues(200, own.Submission.State.Value)
	s.Equal("print(1)", own.Submission.Submission.Value)
	s.Require().Len(own.Submission.TestDetails.Value.Tests, 1, "only samples after the end")
	s.EqualValues(1, own.Submission.TestDetails.Value.Tests[0].TestIndex)
}
