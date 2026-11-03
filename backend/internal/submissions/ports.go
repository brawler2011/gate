package submissions

import (
	"context"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type ContestsUC interface {
	GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error)
}

type OutboxRepo interface {
	eventdata.Writer
}

type ProblemsUC interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
}

type SubmissionsRepo interface {
	BlockSubmission(context.Context, uuid.UUID, *string) error
	ResetSubmissionsState(context.Context, submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error)
	CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
	ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) ([]submissionsmodel.Submission, int32, int64, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error)
}
