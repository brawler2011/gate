package submissions

import (
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSubmissionViewFreeze(t *testing.T) {
	now := time.Now()
	end := now.Add(30 * time.Minute)
	boundary := end.Add(-time.Hour)
	viewer := uuid.New()
	other := uuid.New()
	for _, test := range []struct {
		name      string
		status    contestsmodel.FreezeStatus
		created   time.Time
		author    *uuid.UUID
		moderator bool
		duration  bool
		frozen    bool
	}{
		{"before", "auto", boundary.Add(-time.Nanosecond), &other, false, true, false},
		{"boundary", "auto", boundary, &other, false, true, true},
		{"after", "auto", boundary.Add(time.Nanosecond), &other, false, true, true},
		{"own", "auto", boundary, &viewer, false, true, false},
		{"moderator", "auto", boundary, &other, true, true, false},
		{"forced without boundary", "frozen", boundary.Add(-time.Hour), &other, false, false, true},
		{"unfrozen", "unfrozen", boundary, &other, false, true, false},
		{"missing author", "auto", boundary, nil, false, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			contest := contestsmodel.Contest{Settings: contestsmodel.ContestSettings{FreezeStatus: test.status, Sources: contestsmodel.Same(contestsmodel.ScopePublic), Reports: contestsmodel.Same(contestsmodel.ScopePublic), Feedback: contestsmodel.Same(contestsmodel.FeedbackFull)}}
			if test.duration {
				contest.Settings.FreezeDurationMinutes = new(int32(60))
			}
			if test.duration {
				contest.EndTime = &end
			}
			role := accessmodel.ContestParticipant
			if test.moderator {
				role = accessmodel.ContestModerator
			}
			view := SubmissionView(viewerAccess(viewer, contest, role, now), test.author, test.created, submissionsmodel.KindContest)
			require.Equal(t, test.frozen, view.Hidden())
			require.Equal(t, !test.frozen, view.Source)
			require.Equal(t, !test.frozen, view.Results.Tests())
		})
	}
	futureEnd := now.Add(2 * time.Hour)
	contest := contestsmodel.Contest{EndTime: &futureEnd, Visibility: contestsmodel.ContestVisibilityPublic, Settings: contestsmodel.ContestSettings{FreezeStatus: contestsmodel.FreezeStatusAuto, FreezeDurationMinutes: new(int32(60)),
		Submissions: contestsmodel.Same(contestsmodel.ScopePublic), Feedback: contestsmodel.Same(contestsmodel.FeedbackVerdict)}}
	require.False(t, SubmissionView(viewerAccess(viewer, contest, "", now), &other, futureEnd, submissionsmodel.KindContest).Hidden())
}

func TestSubmissionReportLevels(t *testing.T) {
	compile := "warning"
	details := &submissionsmodel.SubmissionTestDetails{CompilerOutput: &compile, Tests: []submissionsmodel.TestDetailItem{
		{TestIndex: 1, Verdict: "OK", Sample: true}, {TestIndex: 2, Verdict: "OK"}, {TestIndex: 3, Verdict: "WA"},
	}, FailedTestDetails: &submissionsmodel.FailedTestDetail{TestIndex: 3, Input: "secret"}}
	for _, test := range []struct {
		level  contestsmodel.FeedbackLevel
		tests  []int32
		failed bool
	}{
		{contestsmodel.FeedbackVerdict, nil, false},
		{contestsmodel.FeedbackSamples, []int32{1}, false},
		{contestsmodel.FeedbackTests, []int32{1, 2, 3}, false},
		{contestsmodel.FeedbackFull, []int32{1, 2, 3}, true},
	} {
		t.Run(string(test.level), func(t *testing.T) {
			report := SubmissionVisibility{Results: test.level}.Report(details)
			require.Equal(t, &compile, report.CompilerOutput)
			var tests []int32
			for _, item := range report.Tests {
				tests = append(tests, item.TestIndex)
			}
			require.Equal(t, test.tests, tests)
			require.Equal(t, test.failed, report.FailedTestDetails != nil)
		})
	}
	require.Nil(t, SubmissionVisibility{Results: contestsmodel.FeedbackHidden}.Report(details))
	require.Len(t, details.Tests, 3, "trimming must not change the stored report")
}

func TestSubmissionReportGroupFeedback(t *testing.T) {
	failed := int32(4)
	sub := submissionsmodel.Submission{FailedTest: &failed, TestDetails: &submissionsmodel.SubmissionTestDetails{Tests: []submissionsmodel.TestDetailItem{
		{TestIndex: 1, Verdict: "OK", Subtask: "complete"},
		{TestIndex: 2, Verdict: "OK", Subtask: "icpc"}, {TestIndex: 3, Verdict: "WA", Subtask: "icpc"}, {TestIndex: 4, Verdict: "WA", Subtask: "icpc"},
		{TestIndex: 5, Verdict: "WA", Subtask: "points"}, {TestIndex: 6, Verdict: "OK", Subtask: "none"},
	}, Subtasks: []submissionsmodel.SubtaskResult{{Name: "complete", Feedback: "complete"}, {Name: "icpc", Feedback: "icpc"}, {Name: "points", Feedback: "points"}, {Name: "none", Feedback: "none"}},
		FailedTestDetails: &submissionsmodel.FailedTestDetail{TestIndex: 3}}}
	now := time.Now()
	author := uuid.New()
	for _, test := range []struct {
		name      string
		end       time.Duration
		role      accessmodel.ContestRole
		tests     []int32
		subtasks  []string
		failed    bool
		failedNum *int32
	}{
		{"during", time.Hour, accessmodel.ContestParticipant, []int32{1, 2, 3}, []string{"complete", "icpc", "points"}, false, &failed},
		{"after", -time.Hour, accessmodel.ContestParticipant, []int32{1, 2, 3, 4, 5, 6}, []string{"complete", "icpc", "points", "none"}, true, &failed},
		{"moderator", time.Hour, accessmodel.ContestModerator, []int32{1, 2, 3, 4, 5, 6}, []string{"complete", "icpc", "points", "none"}, true, &failed},
	} {
		t.Run(test.name, func(t *testing.T) {
			end := now.Add(test.end)
			contest := contestsmodel.Contest{EndTime: &end, Settings: contestsmodel.ContestSettings{Feedback: contestsmodel.Same(contestsmodel.FeedbackFull)}}
			visible := SubmissionView(viewerAccess(author, contest, test.role, now), &author, now.Add(-2*time.Hour), submissionsmodel.KindContest).Apply(sub)
			var tests []int32
			for _, item := range visible.TestDetails.Tests {
				tests = append(tests, item.TestIndex)
			}
			var subtasks []string
			for _, s := range visible.TestDetails.Subtasks {
				subtasks = append(subtasks, s.Name)
			}
			require.Equal(t, test.tests, tests)
			require.Equal(t, test.subtasks, subtasks)
			require.Equal(t, test.failed, visible.TestDetails.FailedTestDetails != nil)
			require.Equal(t, test.failedNum, visible.FailedTest)
		})
	}
	sub.FailedTest = new(int32(5))
	end := now.Add(time.Hour)
	contest := contestsmodel.Contest{EndTime: &end, Settings: contestsmodel.ContestSettings{Feedback: contestsmodel.Same(contestsmodel.FeedbackFull)}}
	require.Nil(t, SubmissionView(viewerAccess(author, contest, accessmodel.ContestParticipant, now), &author, now, submissionsmodel.KindContest).Apply(sub).FailedTest, "a points group hides which test failed")
	require.Len(t, sub.TestDetails.Tests, 6, "trimming must not change the stored report")
}

func viewerAccess(viewer uuid.UUID, contest contestsmodel.Contest, role accessmodel.ContestRole, now time.Time) access.ContestAccess {
	return access.ContestAccess{Actor: accessmodel.User{Id: viewer, Role: accessmodel.UserRoleUser}, Facts: access.ContestFacts{Contest: contest, Role: role, Now: now}}
}
