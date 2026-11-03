package organizations

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type NotificationsUC interface {
	SendEmail(context.Context, notificationsmodel.Notification)
}

type OrganizationsRepo interface {
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Organization, error)
	GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error)
	ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter) ([]organizationsmodel.Organization, int32, error)

	// Member management
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]organizationsmodel.OrganizationMember, error)
	GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.Organization, error)
	ResolveUserOrganizationID(ctx context.Context, userID uuid.UUID, requestedOrgID *uuid.UUID) (uuid.UUID, bool, error)

	// Invitations
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	ListInvitations(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationInvitation, error)

	// Join Requests
	GetPendingJoinRequest(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
	ListJoinRequests(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationJoinRequest, error)
}

type PermissionsUC interface {
	HasOrganizationPermission(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action accessmodel.OrgAction) (bool, error)
}

type TeamsRepo interface {
	GetTeamByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Team, error)

	ListOrganizationTeams(ctx context.Context, orgID uuid.UUID) ([]organizationsmodel.Team, error)

	// Member management
	GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.TeamMember, error)
	ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]organizationsmodel.TeamMember, error)

	// User's teams
	GetUserTeams(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.Team, error)

	// Resource associations
	GetTeamContests(ctx context.Context, teamID uuid.UUID) ([]contestsmodel.Contest, error)
	GetTeamProblems(ctx context.Context, teamID uuid.UUID) ([]problemsmodel.Problem, error)
}

type UsersRepo interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)

	ListExistingUsernamesByPrefix(ctx context.Context, prefix string) ([]string, error)
}
