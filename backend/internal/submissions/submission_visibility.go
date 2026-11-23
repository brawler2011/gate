package submissions

import (
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
}

// SubmissionView hides what the viewer may not see; results after the freeze stay hidden from other non-managers.
func SubmissionView(a access.ContestAccess, author *uuid.UUID, createdAt time.Time) SubmissionVisibility {
	var owner uuid.UUID
	if author != nil {
		owner = *author
	}
	grant := a.Submission(owner)
	view := SubmissionVisibility{Source: grant.Source, Results: grant.Results}
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
	sub.TestDetails = v.Report(sub.TestDetails)
	return sub
}

// Report trims a test report to the visible level without changing the original.
func (v SubmissionVisibility) Report(details *submissionsmodel.SubmissionTestDetails) *submissionsmodel.SubmissionTestDetails {
	if details == nil || v.Results == contestsmodel.FeedbackFull {
		return details
	}
	if v.Hidden() {
		return nil
	}
	report := *details
	switch v.Results {
	case contestsmodel.FeedbackSamples:
		report.Tests = nil
		samples := map[int32]bool{}
		for _, test := range details.Tests {
			if test.Sample {
				report.Tests = append(report.Tests, test)
				samples[test.TestIndex] = true
			}
		}
		if report.FailedTestDetails != nil && !samples[report.FailedTestDetails.TestIndex] {
			report.FailedTestDetails = nil
		}
	case contestsmodel.FeedbackTests:
		report.FailedTestDetails = nil
	default:
		report.Tests = nil
		report.FailedTestDetails = nil
	}
	return &report
}
