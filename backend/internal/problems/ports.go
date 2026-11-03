package problems

import (
	"context"

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
	CreatePackage(ctx context.Context, params *problemsmodel.CreatePackageParams) (problemsmodel.ProblemPackage, error)
	ListPackages(ctx context.Context, problemID uuid.UUID, limit, offset int32) ([]problemsmodel.ProblemPackage, error)
	GetReadyPackage(ctx context.Context, problemID uuid.UUID) (problemsmodel.ProblemPackage, error)
	GetPackageByID(ctx context.Context, id uuid.UUID) (problemsmodel.ProblemPackage, error)
	GetPackageByVersion(ctx context.Context, problemID uuid.UUID, version int32) (problemsmodel.ProblemPackage, error)
	UpdatePackageStatus(ctx context.Context, params *problemsmodel.UpdatePackageStatusParams) error
}

type ProblemsRepo interface {
	CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error
	CreateProblemTests(ctx context.Context, tests problemsmodel.ProblemTests) error
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error)
	GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]problemsmodel.ProblemTest, error)

	AddProblemTeam(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID uuid.UUID) error
	UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role problemsmodel.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error
	WorkspaceRepo
}

// WorkspaceRepo stores only the location of the canonical GFMT package and its
// searchable catalog fields. CommitWorkspace atomically checks the old hash,
// switches the package reference and updates the catalog.
type WorkspaceRepo interface {
	GetWorkspaceHash(ctx context.Context, id uuid.UUID) (string, error)
	CommitWorkspace(ctx context.Context, id uuid.UUID, previousHash, hash string, metadata *problemsmodel.WorkspaceMetadata) error
}
