package organizations

import (
	"context"

	paging "github.com/brawler2011/gate/backend/internal/page"

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
	ResolveUserOrganizationID(ctx context.Context, userID uuid.UUID, requestedOrgID *uuid.UUID) (uuid.UUID, bool, error)

	ListMembersPage(ctx context.Context, orgID uuid.UUID, request paging.Request) ([]organizationsmodel.OrganizationMember, int32, error)

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

	ListTeams(ctx context.Context, filter *organizationsmodel.TeamsFilter, userID uuid.UUID) ([]organizationsmodel.Team, int32, error)

	// Member management
	GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.TeamMember, error)
	ListTeamMembers(ctx context.Context, teamID uuid.UUID, request paging.Request) ([]organizationsmodel.TeamMember, int32, error)

	// Resource associations
	GetTeamContests(ctx context.Context, teamID, viewerID uuid.UUID, request paging.Request) ([]contestsmodel.Contest, int32, error)
	GetTeamProblems(ctx context.Context, teamID, viewerID uuid.UUID, request paging.Request) ([]problemsmodel.Problem, int32, error)
}

type UsersRepo interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)

	ListExistingUsernamesByPrefix(ctx context.Context, prefix string) ([]string, error)
}
