package submissions

import (
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type SubmissionVisibility struct {
	Frozen  bool
	Source  bool
	Details bool
}

// SubmissionView hides what the viewer may not see; results after the freeze stay hidden from other non-managers.
func SubmissionView(a access.ContestAccess, author *uuid.UUID, createdAt time.Time) SubmissionVisibility {
	var owner uuid.UUID
	if author != nil {
		owner = *author
	}
	own := author != nil && owner == a.Actor.Id
	view := SubmissionVisibility{
		Source:  a.CanSubmission(owner, accessmodel.SubmissionView),
		Details: a.CanSubmission(owner, accessmodel.SubmissionViewDetails),
	}
	contest := a.Facts.Contest
	if !own && !a.Can(accessmodel.ContestManage) && contest.IsFrozenAt(a.Facts.Now) {
		boundary := contest.GetFreezeTime()
		view.Frozen = boundary == nil || !createdAt.Before(*boundary)
	}
	if view.Frozen {
		view.Source = false
		view.Details = false
	}
	return view
}

func (v SubmissionVisibility) Apply(sub submissionsmodel.Submission) submissionsmodel.Submission {
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
