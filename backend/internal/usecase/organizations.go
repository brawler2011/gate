package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
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

func (uc *OrganizationsUseCase) UpdateOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID, input *models.UpdateOrganizationInput) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return uc.UpdateOrganization(ctx, org.ID, userID, input)
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

func (uc *OrganizationsUseCase) DeleteOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return uc.DeleteOrganization(ctx, org.ID, userID)
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

func (uc *OrganizationsUseCase) AddMemberByLogin(ctx context.Context, login string, input *models.AddOrganizationMemberInput, requestUserID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	input.OrganizationID = org.ID
	return uc.AddMember(ctx, input, requestUserID)
}

func (uc *OrganizationsUseCase) ListMembers(ctx context.Context, orgID, requestUserID uuid.UUID) ([]models.OrganizationMember, error) {
	user, err := uc.usersRepo.GetUserById(ctx, requestUserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if user.Role != models.UserRoleAdmin {
		_, err := uc.repo.GetMember(ctx, orgID, requestUserID)
		if err != nil {
			if errors.Is(err, pkg.ErrNotFound) {
				return nil, errors.New("access denied: must be a member of organization")
			}
			return nil, fmt.Errorf("get member: %w", err)
		}
	}

	return uc.repo.ListMembers(ctx, orgID)
}

func (uc *OrganizationsUseCase) ListMembersByLogin(ctx context.Context, login string, requestUserID uuid.UUID) ([]models.OrganizationMember, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	return uc.ListMembers(ctx, org.ID, requestUserID)
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

func (uc *OrganizationsUseCase) RemoveMemberByLogin(ctx context.Context, login string, userID, requestUserID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return uc.RemoveMember(ctx, org.ID, userID, requestUserID)
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

func (uc *OrganizationsUseCase) BatchCreateUsers(
	ctx context.Context,
	input models.BatchCreateOrganizationUsersInput,
	requestUserID uuid.UUID,
) (*models.BatchCreateOrganizationUsersResult, error) {
	if input.Count < 1 || input.Count > 500 {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "count must be between 1 and 500")
	}

	prefix := strings.TrimSpace(input.Prefix)
	if prefix == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "prefix cannot be empty")
	}
	if len(prefix) > 20 {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "prefix cannot exceed 20 characters")
	}

	org, err := uc.repo.GetOrganizationByLogin(ctx, input.OrgLogin)
	if err != nil {
		return nil, err
	}

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, requestUserID, models.ActionManageOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	// Calculate expiration date if ttl_days > 0
	var expiresAt *time.Time
	if input.TTLDays != nil && *input.TTLDays > 0 {
		t := time.Now().Add(time.Duration(*input.TTLDays) * 24 * time.Hour)
		expiresAt = &t
	}

	// Find existing usernames matching prefix to determine padding & index
	existingUsernames, err := uc.usersRepo.ListExistingUsernamesByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list existing usernames: %w", err)
	}

	existingSet := make(map[string]struct{}, len(existingUsernames))
	for _, u := range existingUsernames {
		existingSet[strings.ToLower(u)] = struct{}{}
	}

	padding := 2
	if input.Count >= 100 {
		padding = 3
	}

	type pendingUser struct {
		id           uuid.UUID
		username     string
		password     string
		passwordHash string
	}

	pendingUsers := make([]pendingUser, 0, input.Count)
	currentIndex := 1

	for len(pendingUsers) < int(input.Count) {
		formatStr := fmt.Sprintf("%%s%%0%dd", padding)
		candidate := fmt.Sprintf(formatStr, prefix, currentIndex)
		currentIndex++

		if _, exists := existingSet[strings.ToLower(candidate)]; exists {
			continue
		}
		if err := models.UsernameValidate(candidate); err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, fmt.Sprintf("generated username %q is invalid", candidate))
		}

		pwd, err := pkg.GeneratePassword(10)
		if err != nil {
			return nil, fmt.Errorf("generate password: %w", err)
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
		}

		pendingUsers = append(pendingUsers, pendingUser{
			id:           uuid.New(),
			username:     candidate,
			password:     pwd,
			passwordHash: string(hashed),
		})
	}

	// Save all users and org members in a transaction
	err = uc.transactor.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		txUsersRepo := uc.usersRepo.WithTx(tx)
		txOrgRepo := uc.repo.WithTx(tx)

		for _, pu := range pendingUsers {
			err := txUsersRepo.CreateUser(txCtx, models.CreateUserParams{
				Id:           pu.id,
				Username:     pu.username,
				Role:         models.UserRoleUser,
				PasswordHash: pu.passwordHash,
				Email:        nil,
				AvatarUrl:    nil,
				ExpiresAt:    expiresAt,
			})
			if err != nil {
				return fmt.Errorf("create user %s: %w", pu.username, err)
			}

			err = txOrgRepo.AddMember(txCtx, org.ID, pu.id, models.OrgRoleMember)
			if err != nil {
				return fmt.Errorf("add member %s to org: %w", pu.username, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	resultItems := make([]models.BatchCreatedUserItem, len(pendingUsers))
	for i, pu := range pendingUsers {
		resultItems[i] = models.BatchCreatedUserItem{
			ID:        pu.id,
			Username:  pu.username,
			Password:  pu.password,
			ExpiresAt: expiresAt,
		}
	}

	return &models.BatchCreateOrganizationUsersResult{
		Users: resultItems,
	}, nil
}

