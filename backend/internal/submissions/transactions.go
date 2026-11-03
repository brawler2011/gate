package submissions

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/contests"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type SubmissionTxStore interface {
	contests.ScoreboardStore
	LockContest(context.Context, uuid.UUID) error
	ScheduleJob(context.Context, uuid.UUID) (judging.Job, error)
	CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
	Record(context.Context, eventdata.Event) error
	ResetSubmissionsState(ctx context.Context, filter submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error)
	BlockSubmission(ctx context.Context, id uuid.UUID, reason *string) error
}

type SubmissionTransactions interface {
	Within(context.Context, func(SubmissionTxStore) error) error
}
