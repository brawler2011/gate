package interfaces

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

// TODO: group

type ProblemsRepo interface {
	CreateProblem(ctx context.Context, params *models.CreateProblemParams) error
	CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error
	CreateProblemTests(ctx context.Context, tests models.ProblemTests) error
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error
	GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (models.ProblemMember, error)
	GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]models.ProblemTest, error)
	GetProblemTeams(ctx context.Context, problemId uuid.UUID) ([]models.ProblemTeam, error)
	ListProblems(ctx context.Context, filter *models.ProblemsFilter) ([]models.Problem, int32, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error
	UpdateProblemLimits(ctx context.Context, id uuid.UUID, timeLimitMs, memoryLimitMb int) error
}

type ProblemsUC interface {
	CreateProblem(ctx context.Context, input *models.CreateProblemInput) (uuid.UUID, error)
	CreateProblemTests(ctx context.Context, problemId uuid.UUID, tests []models.ProblemTest) error
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error
	GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (models.ProblemMember, error)
	GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]models.ProblemTest, error)
	ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error
	UploadProblemTests(ctx context.Context, problemId uuid.UUID, zipData []byte) error
}
