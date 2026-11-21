package organizations

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"

	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

type TeamTxStore interface {
	access.Loader
	GetTeamByID(context.Context, uuid.UUID) (*organizationsmodel.Team, error)
	GetTeamMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.TeamMember, error)
	GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	UpdateTeam(context.Context, uuid.UUID, *organizationsmodel.UpdateTeamInput) error
	DeleteTeam(context.Context, uuid.UUID) error
	UpdateTeamMemberRole(context.Context, uuid.UUID, uuid.UUID, accessmodel.TeamRole) error
	RemoveTeamMember(context.Context, uuid.UUID, uuid.UUID) error
	CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error)
	AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role accessmodel.TeamRole) error
}

type TeamTransactions interface {
	Within(context.Context, func(TeamTxStore) error) error
}

type OrganizationTxStore interface {
	access.Loader
	GetOrganizationByID(context.Context, uuid.UUID) (*organizationsmodel.Organization, error)
	GetUserById(context.Context, uuid.UUID) (accessmodel.User, error)
	GetPendingInvitation(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	GetPendingJoinRequest(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
	UpdateOrganization(context.Context, uuid.UUID, *organizationsmodel.UpdateOrganizationInput) error
	DeleteOrganization(context.Context, uuid.UUID) error
	UpdateMemberRole(context.Context, uuid.UUID, uuid.UUID, accessmodel.OrganizationRole) error
	RemoveMember(context.Context, uuid.UUID, uuid.UUID) error
	CreateInvitation(context.Context, *organizationsmodel.CreateOrganizationInvitationInput) (*organizationsmodel.OrganizationInvitation, error)
	CreateJoinRequest(context.Context, *organizationsmodel.CreateOrganizationJoinRequestInput) (*organizationsmodel.OrganizationJoinRequest, error)
	ListMembers(context.Context, uuid.UUID) ([]organizationsmodel.OrganizationMember, error)

	CreateNotification(context.Context, *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error)
	CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error)
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role accessmodel.OrganizationRole) error
	CreateUser(ctx context.Context, params identitymodel.CreateUserParams) error
	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus) error
	UpdateJoinRequestStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, reviewedBy *uuid.UUID) error
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	GetJoinRequestByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)
}

type OrganizationTransactions interface {
	Within(context.Context, func(OrganizationTxStore) error) error
}
