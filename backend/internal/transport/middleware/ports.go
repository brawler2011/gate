package middleware

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type AuthUC interface {
	Authenticate(ctx context.Context, sessionID uuid.UUID) (accessmodel.User, error)
}

type ContestsRepo interface {
	GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error)

	// Contest Join Requests

}

type OrganizationsRepo interface {
	GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error)

	// Member management

	// Invitations

	// Join Requests

}

type PermissionsUC interface {
	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action accessmodel.ContestAction) (bool, error)
	HasProblemPermission(ctx context.Context, problemID uuid.UUID, userID uuid.UUID, action accessmodel.ProblemAction) (bool, error)
	HasOrganizationPermission(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action accessmodel.OrgAction) (bool, error)
}

type SubmissionsUC interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
}

type UsersUC interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
}
