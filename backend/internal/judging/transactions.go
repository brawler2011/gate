package judging

import (
	"context"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type JudgeTxStore interface {
	eventdata.Writer
	// CompleteJob must hold the submission row lock through commit.
	CompleteJob(context.Context, judgingmodel.Job) (bool, error)
	FailStalledJob(context.Context, judgingmodel.JobRef) (bool, error)
	RequeueStalledJob(context.Context, judgingmodel.JobRef) (bool, error)
	GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error)
	UpdateSubmission(context.Context, uuid.UUID, *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error)
}

type JudgeTransactions interface {
	Within(context.Context, func(JudgeTxStore) error) error
}
