package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OrganizationsRepo interface {
	CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error)
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
	GetOrganizationByLogin(ctx context.Context, login string) (*models.Organization, error)
	ListOrganizations(ctx context.Context, filter *models.OrganizationFilter) ([]models.Organization, int32, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, input *models.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, id uuid.UUID) error
	
	// Member management
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role models.OrganizationRole) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrganizationMember, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]models.OrganizationMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrganizationRole) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]models.Organization, error)
	
	WithTx(tx pgx.Tx) OrganizationsRepo
}

type OrganizationsUC interface {
	CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error)
	GetOrganization(ctx context.Context, orgID, userID uuid.UUID) (*models.Organization, error)
	GetOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID) (*models.Organization, error)
	ListOrganizations(ctx context.Context, filter *models.OrganizationFilter, userID uuid.UUID) (*models.OrganizationList, error)
	UpdateOrganization(ctx context.Context, orgID, userID uuid.UUID, input *models.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, orgID, userID uuid.UUID) error
	
	// Member management
	AddMember(ctx context.Context, input *models.AddOrganizationMemberInput, requestUserID uuid.UUID) error
	ListMembers(ctx context.Context, orgID, requestUserID uuid.UUID) ([]models.OrganizationMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrganizationRole, requestUserID uuid.UUID) error
	RemoveMember(ctx context.Context, orgID, userID, requestUserID uuid.UUID) error
	GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]models.Organization, error)
}