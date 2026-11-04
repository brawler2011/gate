package judging

import (
	"context"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type JudgeTxStore interface {
	contests.ScoreboardStore
	eventdata.Writer
	CompleteJob(context.Context, judgingmodel.Job) (bool, error)
	GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error)
	UpdateSubmission(context.Context, uuid.UUID, *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error)
}

type JudgeTransactions interface {
	Within(context.Context, func(JudgeTxStore) error) error
}
