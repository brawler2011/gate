package models

import (
	"time"

	"github.com/google/uuid"
)

// SubmissionVisibility is shared by REST and observer presentation.
type SubmissionVisibility struct {
	Frozen  bool
	Source  bool
	Details bool
}

func SubmissionView(contest *Contest, author *uuid.UUID, createdAt time.Time, viewer uuid.UUID, permissions *ContestPermissions, now time.Time) SubmissionVisibility {
	own := author != nil && *author == viewer
	view := SubmissionVisibility{Source: own || permissions.GetOtherUserSubmission, Details: permissions.GetSubmissionDetails}
	if !own && !permissions.ManageContest && contest.IsFrozenAt(now) {
		boundary := contest.GetFreezeTime()
		view.Frozen = boundary == nil || !createdAt.Before(*boundary)
	}
	if view.Frozen {
		view.Source = false
		view.Details = false
	}
	return view
}

func (v SubmissionVisibility) Apply(sub Submission) Submission {
	sub.Frozen = v.Frozen
	sub.SourceHidden = !v.Source
	if !v.Source {
		sub.Submission = ""
	}
	if !v.Details {
		sub.TestDetails = nil
	}
	if v.Frozen {
		sub.FailedTest = nil
		sub.BanReason = nil
	}
	return sub
}
