package submissions

import (
	"context"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type ContestsUC interface {
	GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error)
}

type ProblemsUC interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
}

type SubmissionsRepo interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
	ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) ([]submissionsmodel.Submission, int32, error)
}
