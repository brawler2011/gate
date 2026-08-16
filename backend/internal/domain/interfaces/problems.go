package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/models"
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
	AddProblemTeam(ctx context.Context, problemID, teamID uuid.UUID, permission models.ProblemPermission) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID uuid.UUID, permission models.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID uuid.UUID) error
	ListProblemMembers(ctx context.Context, problemID uuid.UUID) ([]models.ProblemMember, error)
	UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role models.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error
	ListProblems(ctx context.Context, filter *models.ProblemsFilter) ([]models.Problem, int32, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error
	UpdateProblemLimits(ctx context.Context, id uuid.UUID, timeLimitMs, memoryLimitMb int) error
	GetProblemManifest(ctx context.Context, id uuid.UUID) ([]byte, error)
	UpdateProblemManifest(ctx context.Context, id uuid.UUID, manifest []byte) error
	ListDashboardProblems(ctx context.Context, userID uuid.UUID, limit int32) ([]models.DashboardProblem, error)
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
	UpdateProblemLimits(ctx context.Context, id uuid.UUID, timeLimitMs, memoryLimitMb int) error
	UploadProblemTests(ctx context.Context, problemId uuid.UUID, zipData []byte) error
	ListDashboardProblems(ctx context.Context, userID uuid.UUID, limit int32) ([]models.DashboardProblem, error)

	AddProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission models.ProblemPermission) error
	GetProblemTeams(ctx context.Context, problemID, requestUserID uuid.UUID) ([]models.ProblemTeam, error)
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission models.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID) error

	CreateProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role models.ProblemRole) error
	ListProblemMembers(ctx context.Context, problemID, requestUserID uuid.UUID) ([]models.ProblemMember, error)
	UpdateProblemMemberRole(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role models.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID) error
}
