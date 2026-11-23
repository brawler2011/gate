package problems

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"

	"github.com/google/uuid"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
)

type ProblemTxStore interface {
	access.Loader
	GetReadyPackage(context.Context, uuid.UUID) (problemsmodel.ProblemPackage, error)
	GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(context.Context, uuid.UUID, uuid.UUID) (problemsmodel.ProblemMember, error)
	GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	UpdateProblem(context.Context, uuid.UUID, *problemsmodel.ProblemUpdate) error
	DeleteProblem(context.Context, uuid.UUID) error
	UpdateProblemMemberRole(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemRole) error
	RemoveProblemMember(context.Context, uuid.UUID, uuid.UUID) error
	CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error
}

type ProblemTransactions interface {
	Within(context.Context, func(ProblemTxStore) error) error
}
