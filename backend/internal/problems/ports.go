package problems

import (
	"context"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	paging "github.com/brawler2011/gate/backend/internal/page"

	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type OrganizationsRepo interface {
	GetUserOrganizations(context.Context, uuid.UUID) ([]organizationsmodel.Organization, error)
}

type PackagesRepo interface {
	GetReadyPackage(context.Context, uuid.UUID) (problemsmodel.ProblemPackage, error)
	ListPackages(ctx context.Context, problemID uuid.UUID, request paging.Request) ([]problemsmodel.ProblemPackage, int32, error)
}

type ProblemsRepo interface {
	GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error)
	ListProblemMembers(context.Context, uuid.UUID, paging.Request) ([]problemsmodel.ProblemMember, int32, error)
	GetProblemTeams(context.Context, uuid.UUID) ([]problemsmodel.ProblemTeam, error)
	ListProblems(context.Context, *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error)
	ListDashboardProblems(context.Context, uuid.UUID, int32) ([]problemsmodel.DashboardProblem, error)
}

type DraftsRepo interface {
	GetDraft(context.Context, uuid.UUID) (problemsmodel.Draft, error)
}

type ProblemPermissions interface {
	HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error)
}
