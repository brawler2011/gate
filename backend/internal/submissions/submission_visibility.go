package submissions

import (
	"slices"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type SubmissionVisibility struct {
	Source  bool
	Results contestsmodel.FeedbackLevel
	Groups  bool
}

// SubmissionView hides what the viewer may not see; results after the freeze stay hidden from other non-managers.
func SubmissionView(a access.ContestAccess, author *uuid.UUID, createdAt time.Time, kind submissionsmodel.Kind) SubmissionVisibility {
	var owner uuid.UUID
	if author != nil {
		owner = *author
	}
	grant := a.Submission(owner, kind == submissionsmodel.KindVirtual)
	view := SubmissionVisibility{Source: grant.Source, Results: grant.Results, Groups: grant.Groups}
	own := author != nil && owner == a.Actor.Id
	contest := a.Facts.Contest
	if !own && !a.Can(accessmodel.ContestManage) && contest.IsFrozenAt(a.Facts.Now) {
		if boundary := contest.GetFreezeTime(); boundary == nil || !createdAt.Before(*boundary) {
			view.Hide()
		}
	}
	return view
}

func (v *SubmissionVisibility) Hide() {
	v.Source = false
	v.Results = contestsmodel.FeedbackHidden
}

func (v SubmissionVisibility) Hidden() bool { return v.Results == contestsmodel.FeedbackHidden }

// Progress reports whether live test numbers may be shown; they do not say which tests are samples.
func (v SubmissionVisibility) Progress() bool {
	return v.Results == contestsmodel.FeedbackTests || v.Results == contestsmodel.FeedbackFull
}

func (v SubmissionVisibility) Apply(sub submissionsmodel.Submission) submissionsmodel.Submission {
	sub.ResultHidden = v.Hidden()
	sub.SourceHidden = !v.Source
	if !v.Source {
		sub.Submission = ""
	}
	if v.Hidden() {
		sub.FailedTest = nil
		sub.BanReason = nil
	}
	if sub.FailedTest != nil && v.Groups && !submissionsmodel.FeedbackShowsFailure(sub.FailedFeedback()) {
		sub.FailedTest = nil
	}
	sub.TestDetails = v.Report(sub.TestDetails)
	return sub
}

// Report trims a test report to the visible level without changing the original.
func (v SubmissionVisibility) Report(details *submissionsmodel.SubmissionTestDetails) *submissionsmodel.SubmissionTestDetails {
	if details == nil || v.Results == contestsmodel.FeedbackFull && !v.Groups {
		return details
	}
	if v.Hidden() {
		return nil
	}
	report := *details
	if v.Groups {
		report = limitGroups(report)
	}
	switch v.Results {
	case contestsmodel.FeedbackSamples:
		tests := report.Tests
		report.Tests = nil
		samples := map[int32]bool{}
		for _, test := range tests {
			if test.Sample {
				report.Tests = append(report.Tests, test)
				samples[test.TestIndex] = true
			}
		}
		if report.FailedTestDetails != nil && !samples[report.FailedTestDetails.TestIndex] {
			report.FailedTestDetails = nil
		}
		report.Subtasks = nil
	case contestsmodel.FeedbackTests:
		report.FailedTestDetails = nil
	case contestsmodel.FeedbackFull:
	default:
		report.Tests = nil
		report.Subtasks = nil
		report.FailedTestDetails = nil
	}
	return &report
}

type feedbackPolicy string

func (p feedbackPolicy) complete() bool { return p == "" || p == "complete" }

// limitGroups applies each group's feedback policy to a copy of the report.
func limitGroups(report submissionsmodel.SubmissionTestDetails) submissionsmodel.SubmissionTestDetails {
	policies := map[string]feedbackPolicy{}
	report.Subtasks = slices.DeleteFunc(slices.Clone(report.Subtasks), func(s submissionsmodel.SubtaskResult) bool {
		policies[s.Name] = feedbackPolicy(s.Feedback)
		return s.Feedback == "none"
	})
	failed := map[string]bool{}
	var tests []submissionsmodel.TestDetailItem
	for _, t := range report.Tests {
		if report.FailedTestDetails != nil && t.TestIndex == report.FailedTestDetails.TestIndex && !policies[t.Subtask].complete() {
			report.FailedTestDetails = nil
		}
		switch p := policies[t.Subtask]; {
		case p.complete():
		case p == "icpc":
			if failed[t.Subtask] {
				continue
			}
			failed[t.Subtask] = t.Verdict != "OK"
		default:
			continue
		}
		tests = append(tests, t)
	}
	report.Tests = tests
	return report
}
