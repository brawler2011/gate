package organizations

import (
	"context"

	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

type TeamTxStore interface {
	CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error)
	AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error
}

type TeamTransactions interface {
	Within(context.Context, func(TeamTxStore) error) error
}

type OrganizationTxStore interface {
	CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error)
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error
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
