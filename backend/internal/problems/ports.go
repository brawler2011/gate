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

	// Member management

	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)

	// Invitations
}

type PackagesRepo interface {
	GetReadyPackage(context.Context, uuid.UUID) (problemsmodel.ProblemPackage, error)
	ListPackages(ctx context.Context, problemID uuid.UUID, request paging.Request) ([]problemsmodel.ProblemPackage, int32, error)
}

type ProblemsRepo interface {
	GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error)
}

type ProblemPermissions interface {
	HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error)
}
