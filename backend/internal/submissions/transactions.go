package submissions

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	"github.com/brawler2011/gate/backend/internal/contests"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type SubmissionTxStore interface {
	access.Loader
	GetContestProblem(context.Context, contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error)
	GetContestUserProblemBlock(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error)
	contests.ScoreboardStore
	LockSubmissionTuple(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	ScheduleJob(context.Context, uuid.UUID) (judgingmodel.Job, error)
	CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
	eventdata.Writer
	ResetSubmissionsState(ctx context.Context, filter submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error)
	BlockSubmission(ctx context.Context, id uuid.UUID, reason *string) error
}

type SubmissionTransactions interface {
	Within(context.Context, func(SubmissionTxStore) error) error
}
