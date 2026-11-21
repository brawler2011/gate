package core

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type ProblemsRepo interface {
	CreateProblem(ctx context.Context, params *problemsmodel.CreateProblemParams) error
	CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error)
	GetProblemTeams(ctx context.Context, problemId uuid.UUID) ([]problemsmodel.ProblemTeam, error)
	AddProblemTeam(ctx context.Context, problemID, teamID uuid.UUID, permission accessmodel.ProblemRole) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID uuid.UUID, permission accessmodel.ProblemRole) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID uuid.UUID) error
	UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role accessmodel.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error
}
