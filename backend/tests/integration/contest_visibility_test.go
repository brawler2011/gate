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
	path := corev1.UpdateContestParams{ContestID: contestID}
	s.Require().NoError(s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{
		FeedbackDuring: corev1.NewOptFeedbackLevel(corev1.FeedbackLevelHidden), FeedbackAfter: corev1.NewOptFeedbackLevel(corev1.FeedbackLevelSamples),
		ScoreboardDuring: corev1.NewOptContestScope(corev1.ContestScopeParticipant)}, path))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{
		ContestId: contestID, ProblemId: problem.ID, PackageId: s.createDummyProblemPackage(problem.ID, org.ID)}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID, UserId: participant.Id, Role: accessmodel.ContestParticipant}))
	as := withTestUser(s.ctx, participant.Id)
	created, err := s.client.CreateSubmission(as, &corev1.CreateSubmissionRequestModel{Submission: "print(1)"},
		corev1.CreateSubmissionParams{ProblemID: problem.ID, ContestID: contestID, Language: "python3"})
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
	_, err = s.client.ListContestSubmissions(as, corev1.ListContestSubmissionsParams{ContestID: contestID,
		UserId: corev1.NewOptUUID(participant.Id), State: corev1.NewOptInt32(200)})
	s.Equal(http.StatusForbidden, status(err), "a state filter would reveal the verdict")
	_, err = s.client.ListSubmissions(as, corev1.ListSubmissionsParams{UserId: corev1.NewOptUUID(participant.Id), State: corev1.NewOptInt32(200)})
	s.Equal(http.StatusForbidden, status(err), "nor may it span contests")
	_, err = s.client.GetContestScoreboard(as, corev1.GetContestScoreboardParams{ContestID: contestID})
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

// A group whose policy shows only points must not reveal which of its tests failed, in lists either.
func (s *IntegrationTestSuite) TestGroupFeedbackHidesFailedTestInLists() {
	admin := s.createUser("admin_groups_"+uuid.NewString()[:8], accessmodel.UserRoleAdmin)
	participant := s.createUser("participant_groups_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("groups-"+uuid.NewString()[:8], "Group Feedback", admin.Id)
	problem, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{
		Title: "Groups", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Title: "Groups",
		Login: "groups", Visibility: contestsmodel.ContestVisibilityPublic}, admin.Id))
	setEnd := func(end time.Time) {
		start := time.Now().Add(-2 * time.Hour)
		s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID,
			StartTime: &start, StartTimeSet: true, EndTime: &end, EndTimeSet: true}))
	}
	setEnd(time.Now().Add(time.Hour))
	s.Require().NoError(s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{
		FeedbackDuring: corev1.NewOptFeedbackLevel(corev1.FeedbackLevelFull)}, corev1.UpdateContestParams{ContestID: contestID}))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{
		ContestId: contestID, ProblemId: problem.ID, PackageId: s.createDummyProblemPackage(problem.ID, org.ID)}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID, UserId: participant.Id, Role: accessmodel.ContestParticipant}))
	as := withTestUser(s.ctx, participant.Id)
	created, err := s.client.CreateSubmission(as, &corev1.CreateSubmissionRequestModel{Submission: "print(1)"},
		corev1.CreateSubmissionParams{ProblemID: problem.ID, ContestID: contestID, Language: "python3"})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `UPDATE submissions SET state=106, test_details=$2 WHERE id=$1`, created.ID,
		`{"failed_test":2,"tests":[{"test_index":1,"verdict":"OK","group":""},{"test_index":2,"verdict":"WA","group":"secret"}],
		"groups":[{"name":""},{"name":"secret","feedback":"points"}]}`)
	s.Require().NoError(err)
	listed := func() corev1.OptNilInt32 {
		list, err := s.client.ListContestSubmissions(as, corev1.ListContestSubmissionsParams{ContestID: contestID, UserId: corev1.NewOptUUID(participant.Id)})
		s.Require().NoError(err)
		s.Require().Len(list.Submissions, 1)
		return list.Submissions[0].FailedTest
	}

	s.False(listed().IsSet())
	own, err := s.client.GetSubmission(as, corev1.GetSubmissionParams{SubmissionID: created.ID})
	s.Require().NoError(err)
	s.False(own.Submission.FailedTest.IsSet())
	s.Require().Len(own.Submission.TestDetails.Value.Tests, 1, "tests of a points group stay hidden")
	s.EqualValues(1, own.Submission.TestDetails.Value.Tests[0].TestIndex)

	setEnd(time.Now().Add(-time.Minute))
	s.EqualValues(2, listed().Value)
}
