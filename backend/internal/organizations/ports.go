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
	Notify(ctx context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error)
}

type OrganizationsRepo interface {
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Organization, error)
	GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error)
	ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter) ([]organizationsmodel.Organization, int32, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, input *organizationsmodel.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, id uuid.UUID) error

	// Member management
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]organizationsmodel.OrganizationMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.Organization, error)
	ResolveUserOrganizationID(ctx context.Context, userID uuid.UUID, requestedOrgID *uuid.UUID) (uuid.UUID, bool, error)

	// Invitations
	CreateInvitation(ctx context.Context, input *organizationsmodel.CreateOrganizationInvitationInput) (*organizationsmodel.OrganizationInvitation, error)
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	GetPendingInvitation(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	ListInvitations(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationInvitation, error)

	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus) error

	// Join Requests
	CreateJoinRequest(ctx context.Context, input *organizationsmodel.CreateOrganizationJoinRequestInput) (*organizationsmodel.OrganizationJoinRequest, error)
	GetJoinRequestByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
	GetPendingJoinRequest(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
	ListJoinRequests(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationJoinRequest, error)

	UpdateJoinRequestStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, reviewedBy *uuid.UUID) error
}

type PermissionsUC interface {
	HasOrganizationPermission(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action accessmodel.OrgAction) (bool, error)
}

type TeamsRepo interface {
	GetTeamByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Team, error)

	ListOrganizationTeams(ctx context.Context, orgID uuid.UUID) ([]organizationsmodel.Team, error)
	UpdateTeam(ctx context.Context, id uuid.UUID, input *organizationsmodel.UpdateTeamInput) error
	DeleteTeam(ctx context.Context, id uuid.UUID) error

	// Member management
	AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error
	GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.TeamMember, error)
	ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]organizationsmodel.TeamMember, error)
	UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error
	RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error

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
