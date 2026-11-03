package access

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type ContestsRepo interface {
	GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error)

	GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error)

	GetContestTeams(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestTeam, error)
}

type OrganizationsRepo interface {

	// Member management

	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)

	// Invitations
}

type ProblemsRepo interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error)

	GetProblemTeams(ctx context.Context, problemId uuid.UUID) ([]problemsmodel.ProblemTeam, error)

	WorkspaceRepo
}

type TeamsRepo interface {
	// User's teams

	GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]organizationsmodel.Team, error)

	// Resource associations

}

type UsersUC interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
}

// WorkspaceRepo stores only the location of the canonical GFMT package and its
// searchable catalog fields. CommitWorkspace atomically checks the old hash,
// switches the package reference and updates the catalog.
type WorkspaceRepo interface {
}
