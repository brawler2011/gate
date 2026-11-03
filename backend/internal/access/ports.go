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
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)
}

type ProblemsRepo interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error)

	GetProblemTeams(ctx context.Context, problemId uuid.UUID) ([]problemsmodel.ProblemTeam, error)
}

type TeamsRepo interface {
	GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]organizationsmodel.Team, error)
}

type UsersReader interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
}
