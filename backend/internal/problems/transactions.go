package problems

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"

	"github.com/google/uuid"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
)

type ProblemTxStore interface {
	GetUserById(context.Context, uuid.UUID) (accessmodel.User, error)
	GetReadyPackage(context.Context, uuid.UUID) (problemsmodel.ProblemPackage, error)
	HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error)
	GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error)
	GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	UpdateProblem(context.Context, uuid.UUID, *problemsmodel.ProblemUpdate) error
	DeleteProblem(context.Context, uuid.UUID) error
	AddProblemTeam(context.Context, uuid.UUID, uuid.UUID, problemsmodel.ProblemPermission) error
	UpdateProblemTeamPermission(context.Context, uuid.UUID, uuid.UUID, problemsmodel.ProblemPermission) error
	RemoveProblemTeam(context.Context, uuid.UUID, uuid.UUID) error
	UpdateProblemMemberRole(context.Context, uuid.UUID, uuid.UUID, problemsmodel.ProblemRole) error
	RemoveProblemMember(context.Context, uuid.UUID, uuid.UUID) error
	CreateProblem(ctx context.Context, params *problemsmodel.CreateProblemParams) error
	CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error
}

type ProblemTransactions interface {
	Within(context.Context, func(ProblemTxStore) error) error
}
