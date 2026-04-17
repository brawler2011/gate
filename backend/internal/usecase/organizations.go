package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OrganizationsUseCase struct {
	repo          interfaces.OrganizationsRepo
	usersRepo     interfaces.UsersRepo
	permissionsUC *PermissionsUseCase
	transactor    interfaces.Transactor
}

func NewOrganizationsUseCase(
	repo interfaces.OrganizationsRepo,
	usersRepo interfaces.UsersRepo,
	permissionsUC *PermissionsUseCase,
	transactor interfaces.Transactor,
) *OrganizationsUseCase {
	return &OrganizationsUseCase{
		repo:          repo,
		usersRepo:     usersRepo,
		permissionsUC: permissionsUC,
		transactor:    transactor,
	}
}

func (uc *OrganizationsUseCase) CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error) {
	// Verify creator exists
	creator, err := uc.usersRepo.GetUserById(ctx, input.CreatorID)
	if err != nil {
		return nil, fmt.Errorf("get creator: %w", err)
	}

	// Only admins can create organizations
	if creator.Role != models.UserRoleAdmin {
		return nil, errors.New("only admins can create organizations")
	}

	var org *models.Organization
	err = uc.transactor.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		// Create organization
		org, err = uc.repo.CreateOrganization(txCtx, input)
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}

		// Add creator as owner
		err = uc.repo.AddMember(txCtx, org.ID, input.CreatorID, models.OrgRoleOwner)
		if err != nil {
			return fmt.Errorf("add creator as owner: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return org, nil
}

func (uc *OrganizationsUseCase) GetOrganization(ctx context.Context, orgID, userID uuid.UUID) (*models.Organization, error) {
	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, userID, models.ActionViewOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, errors.New("access denied")
	}

	return uc.repo.GetOrganizationByID(ctx, orgID)
}

func (uc *OrganizationsUseCase) GetOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID) (*models.Organization, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return nil, err
	}

	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, userID, models.ActionViewOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, errors.New("access denied")
	}

	return org, nil
}

func (uc *OrganizationsUseCase) ListOrganizations(ctx context.Context, filter *models.OrganizationFilter, userID uuid.UUID) (*models.OrganizationList, error) {
	// List all organizations (visibility filtering happens at repo level or we filter here)
	orgs, total, err := uc.repo.ListOrganizations(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Filter based on user access
	accessibleOrgs := make([]models.Organization, 0, len(orgs))
	for _, org := range orgs {
		hasAccess, _ := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, userID, models.ActionViewOrganization)
		if hasAccess {
			accessibleOrgs = append(accessibleOrgs, org)
		}
	}

	totalPages := int32(0)
	if filter.PageSize > 0 {
		totalPages = (total + filter.PageSize - 1) / filter.PageSize
	}

	return &models.OrganizationList{
		Organizations: accessibleOrgs,
		Pagination: models.Pagination{
			Page:  filter.Page,
			Total: totalPages,
		},
	}, nil
}

func (uc *OrganizationsUseCase) UpdateOrganization(ctx context.Context, orgID, userID uuid.UUID, input *models.UpdateOrganizationInput) error {
	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, userID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return errors.New("access denied")
	}

	return uc.repo.UpdateOrganization(ctx, orgID, input)
}

func (uc *OrganizationsUseCase) DeleteOrganization(ctx context.Context, orgID, userID uuid.UUID) error {
	// Check permissions - only owners can delete
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, userID, models.ActionDeleteOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return errors.New("access denied: only owners can delete organizations")
	}

	return uc.repo.DeleteOrganization(ctx, orgID)
}

// Member management

func (uc *OrganizationsUseCase) AddMember(ctx context.Context, input *models.AddOrganizationMemberInput, requestUserID uuid.UUID) error {
	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, input.OrganizationID, requestUserID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return errors.New("access denied")
	}

	// Verify user exists
	_, err = uc.usersRepo.GetUserById(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	return uc.repo.AddMember(ctx, input.OrganizationID, input.UserID, input.Role)
}

func (uc *OrganizationsUseCase) ListMembers(ctx context.Context, orgID, requestUserID uuid.UUID) ([]models.OrganizationMember, error) {
	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, requestUserID, models.ActionViewOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, errors.New("access denied")
	}

	return uc.repo.ListMembers(ctx, orgID)
}

func (uc *OrganizationsUseCase) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrganizationRole, requestUserID uuid.UUID) error {
	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, requestUserID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return errors.New("access denied")
	}

	return uc.repo.UpdateMemberRole(ctx, orgID, userID, role)
}

func (uc *OrganizationsUseCase) RemoveMember(ctx context.Context, orgID, userID, requestUserID uuid.UUID) error {
	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, requestUserID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return errors.New("access denied")
	}

	return uc.repo.RemoveMember(ctx, orgID, userID)
}

func (uc *OrganizationsUseCase) GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]models.Organization, error) {
	return uc.repo.GetUserOrganizations(ctx, userID)
}

func (uc *OrganizationsUseCase) ResolveUserOrganizationID(ctx context.Context, userID uuid.UUID, requestedOrgID *uuid.UUID) (uuid.UUID, error) {
	orgID, found, err := uc.repo.ResolveUserOrganizationID(ctx, userID, requestedOrgID)
	if err != nil {
		return uuid.Nil, err
	}

	if !found {
		if requestedOrgID != nil {
			return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "organization not found or user is not a member")
		}

		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "user has no organizations")
	}

	return orgID, nil
}
