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
	repo            interfaces.OrganizationsRepo
	usersRepo       interfaces.UsersRepo
	permissionsUC   *PermissionsUseCase
	transactor      interfaces.Transactor
	notificationsUC interfaces.NotificationsUC
}

func NewOrganizationsUseCase(
	repo interfaces.OrganizationsRepo,
	usersRepo interfaces.UsersRepo,
	permissionsUC *PermissionsUseCase,
	transactor interfaces.Transactor,
	notificationsUC interfaces.NotificationsUC,
) *OrganizationsUseCase {
	return &OrganizationsUseCase{
		repo:            repo,
		usersRepo:       usersRepo,
		permissionsUC:   permissionsUC,
		transactor:      transactor,
		notificationsUC: notificationsUC,
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
		org, err = uc.repo.WithTx(tx).CreateOrganization(txCtx, input)
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}

		// Add creator as owner
		err = uc.repo.WithTx(tx).AddMember(txCtx, org.ID, input.CreatorID, models.OrgRoleOwner)
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
	// List all organizations
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

// Invitations

func (uc *OrganizationsUseCase) InviteMember(
	ctx context.Context,
	orgLogin string,
	targetUserID uuid.UUID,
	role models.OrganizationRole,
	inviterID uuid.UUID,
) (*models.OrganizationInvitation, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, inviterID, models.ActionManageOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	// Verify target user exists
	_, err = uc.usersRepo.GetUserById(ctx, targetUserID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "user not found")
	}

	inviterUser, err := uc.usersRepo.GetUserById(ctx, inviterID)
	if err != nil {
		return nil, fmt.Errorf("get inviter user: %w", err)
	}

	// Check if already a member
	_, err = uc.repo.GetMember(ctx, org.ID, targetUserID)
	if err == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "пользователь уже является участником организации")
	}

	// Check if there is already a pending invitation
	pending, err := uc.repo.GetPendingInvitation(ctx, org.ID, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("get pending invitation: %w", err)
	}
	if pending != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "приглашение этому пользователю уже отправлено")
	}

	invitation, err := uc.repo.CreateInvitation(ctx, &models.CreateOrganizationInvitationInput{
		OrganizationID: org.ID,
		UserID:         targetUserID,
		InviterID:      inviterID,
		Role:           role,
	})
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	// Send in-app notification & async email
	if uc.notificationsUC != nil {
		link := "/notifications"
		_, _ = uc.notificationsUC.Notify(ctx, &models.CreateNotificationInput{
			UserID: targetUserID,
			Type:   models.NotificationTypeOrgInvitation,
			Title:  fmt.Sprintf("Приглашение в организацию %s", org.Name),
			Body:   fmt.Sprintf("Пользователь @%s пригласил вас вступить в организацию %s (роль: %s)", inviterUser.Username, org.Name, role),
			Link:   &link,
			Data: map[string]interface{}{
				"organization_id":    org.ID.String(),
				"organization_name":  org.Name,
				"organization_login": org.Login,
				"role":               string(role),
				"inviter_id":         inviterID.String(),
				"inviter_username":   inviterUser.Username,
				"invitation_id":      invitation.ID.String(),
			},
		})
	}

	return invitation, nil
}

func (uc *OrganizationsUseCase) ListInvitations(ctx context.Context, orgLogin string, requestUserID uuid.UUID) ([]models.OrganizationInvitation, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
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

	status := string(models.RequestStatusPending)
	return uc.repo.ListInvitations(ctx, org.ID, &status)
}

func (uc *OrganizationsUseCase) CancelInvitation(ctx context.Context, orgLogin string, invitationID, requestUserID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, requestUserID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	inv, err := uc.repo.GetInvitationByID(ctx, invitationID)
	if err != nil {
		return err
	}
	if inv.OrganizationID != org.ID {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invitation does not belong to this organization")
	}

	return uc.repo.UpdateInvitationStatus(ctx, invitationID, models.RequestStatusCanceled)
}

func (uc *OrganizationsUseCase) AcceptInvitation(ctx context.Context, invitationID, requestUserID uuid.UUID) error {
	inv, err := uc.repo.GetInvitationByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if inv.UserID != requestUserID {
		return pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}
	if inv.Status != models.RequestStatusPending {
		return pkg.Wrap(pkg.ErrBadInput, nil, "приглашение уже не активно")
	}

	return uc.transactor.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		txRepo := uc.repo.WithTx(tx)

		if err := txRepo.AddMember(txCtx, inv.OrganizationID, inv.UserID, inv.Role); err != nil {
			return fmt.Errorf("add member: %w", err)
		}

		if err := txRepo.UpdateInvitationStatus(txCtx, inv.ID, models.RequestStatusAccepted); err != nil {
			return fmt.Errorf("update invitation status: %w", err)
		}

		return nil
	})
}

func (uc *OrganizationsUseCase) DeclineInvitation(ctx context.Context, invitationID, requestUserID uuid.UUID) error {
	inv, err := uc.repo.GetInvitationByID(ctx, invitationID)
	if err != nil {
		return err
	}

	if inv.UserID != requestUserID {
		return pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}
	if inv.Status != models.RequestStatusPending {
		return pkg.Wrap(pkg.ErrBadInput, nil, "приглашение уже не активно")
	}

	return uc.repo.UpdateInvitationStatus(ctx, inv.ID, models.RequestStatusDeclined)
}

// Join Requests

func (uc *OrganizationsUseCase) CreateJoinRequest(
	ctx context.Context,
	orgLogin string,
	userID uuid.UUID,
	message *string,
) (*models.OrganizationJoinRequest, bool, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, false, err
	}

	// Check if already a member
	_, err = uc.repo.GetMember(ctx, org.ID, userID)
	if err == nil {
		return nil, false, pkg.Wrap(pkg.ErrBadInput, nil, "вы уже состоите в этой организации")
	}

	applicantUser, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return nil, false, fmt.Errorf("get applicant user: %w", err)
	}

	if org.JoinPolicy == models.OrgJoinPolicyInviteOnly {
		return nil, false, pkg.Wrap(pkg.ErrBadInput, nil, "вступление в организацию только по приглашениям")
	}

	if org.JoinPolicy == models.OrgJoinPolicyOpen {
		// Immediately add member
		if err := uc.repo.AddMember(ctx, org.ID, userID, models.OrgRoleMember); err != nil {
			return nil, false, fmt.Errorf("add member: %w", err)
		}
		return nil, true, nil
	}

	// By Request
	pending, err := uc.repo.GetPendingJoinRequest(ctx, org.ID, userID)
	if err != nil {
		return nil, false, fmt.Errorf("get pending request: %w", err)
	}
	if pending != nil {
		return nil, false, pkg.Wrap(pkg.ErrBadInput, nil, "заявка на вступление уже подана и ожидает рассмотрения")
	}

	req, err := uc.repo.CreateJoinRequest(ctx, &models.CreateOrganizationJoinRequestInput{
		OrganizationID: org.ID,
		UserID:         userID,
		Message:        message,
	})
	if err != nil {
		return nil, false, fmt.Errorf("create join request: %w", err)
	}

	// Notify all org owners and admins
	if uc.notificationsUC != nil {
		members, err := uc.repo.ListMembers(ctx, org.ID)
		if err == nil {
			link := fmt.Sprintf("/%s/settings/members", org.Login)
			for _, m := range members {
				if m.Role == models.OrgRoleOwner || m.Role == models.OrgRoleAdmin {
					_, _ = uc.notificationsUC.Notify(ctx, &models.CreateNotificationInput{
						UserID: m.UserID,
						Type:   models.NotificationTypeOrgJoinRequest,
						Title:  fmt.Sprintf("Новая заявка в организацию %s", org.Name),
						Body:   fmt.Sprintf("Пользователь @%s подал заявку на вступление в организацию %s", applicantUser.Username, org.Name),
						Link:   &link,
						Data: map[string]interface{}{
							"organization_id":    org.ID.String(),
							"organization_name":  org.Name,
							"organization_login": org.Login,
							"applicant_id":       userID.String(),
							"applicant_username": applicantUser.Username,
							"request_id":         req.ID.String(),
						},
					})
				}
			}
		}
	}

	return req, false, nil
}

func (uc *OrganizationsUseCase) ListJoinRequests(ctx context.Context, orgLogin string, requestUserID uuid.UUID) ([]models.OrganizationJoinRequest, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
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

	status := string(models.RequestStatusPending)
	return uc.repo.ListJoinRequests(ctx, org.ID, &status)
}

func (uc *OrganizationsUseCase) CancelJoinRequest(ctx context.Context, orgLogin string, requestUserID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	req, err := uc.repo.GetPendingJoinRequest(ctx, org.ID, requestUserID)
	if err != nil {
		return err
	}
	if req == nil {
		return pkg.Wrap(pkg.ErrNotFound, nil, "активная заявка не найдена")
	}

	return uc.repo.UpdateJoinRequestStatus(ctx, req.ID, models.RequestStatusCanceled, nil)
}

func (uc *OrganizationsUseCase) ApproveJoinRequest(
	ctx context.Context,
	orgLogin string,
	requestID, reviewerID uuid.UUID,
	role models.OrganizationRole,
) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, reviewerID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	req, err := uc.repo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.OrganizationID != org.ID {
		return pkg.Wrap(pkg.ErrBadInput, nil, "request does not belong to this organization")
	}
	if req.Status != models.RequestStatusPending {
		return pkg.Wrap(pkg.ErrBadInput, nil, "заявка уже рассмотрена")
	}

	if role == "" {
		role = models.OrgRoleMember
	}

	err = uc.transactor.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		txRepo := uc.repo.WithTx(tx)

		if err := txRepo.AddMember(txCtx, req.OrganizationID, req.UserID, role); err != nil {
			return fmt.Errorf("add member: %w", err)
		}

		if err := txRepo.UpdateJoinRequestStatus(txCtx, req.ID, models.RequestStatusApproved, &reviewerID); err != nil {
			return fmt.Errorf("update join request status: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Notify applicant
	if uc.notificationsUC != nil {
		link := fmt.Sprintf("/%s", org.Login)
		_, _ = uc.notificationsUC.Notify(ctx, &models.CreateNotificationInput{
			UserID: req.UserID,
			Type:   models.NotificationTypeOrgJoinApproved,
			Title:  fmt.Sprintf("Заявка в организацию %s одобрена", org.Name),
			Body:   fmt.Sprintf("Ваша заявка на вступление в организацию %s была одобрена. Добро пожаловать!", org.Name),
			Link:   &link,
			Data: map[string]interface{}{
				"organization_id":    org.ID.String(),
				"organization_name":  org.Name,
				"organization_login": org.Login,
				"request_id":         req.ID.String(),
			},
		})
	}

	return nil
}

func (uc *OrganizationsUseCase) RejectJoinRequest(ctx context.Context, orgLogin string, requestID, reviewerID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, reviewerID, models.ActionManageOrganization)
	if err != nil {
		return fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	req, err := uc.repo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.OrganizationID != org.ID {
		return pkg.Wrap(pkg.ErrBadInput, nil, "request does not belong to this organization")
	}
	if req.Status != models.RequestStatusPending {
		return pkg.Wrap(pkg.ErrBadInput, nil, "заявка уже рассмотрена")
	}

	if err := uc.repo.UpdateJoinRequestStatus(ctx, req.ID, models.RequestStatusRejected, &reviewerID); err != nil {
		return fmt.Errorf("update join request status: %w", err)
	}

	// Notify applicant
	if uc.notificationsUC != nil {
		link := fmt.Sprintf("/%s", org.Login)
		_, _ = uc.notificationsUC.Notify(ctx, &models.CreateNotificationInput{
			UserID: req.UserID,
			Type:   models.NotificationTypeOrgJoinRejected,
			Title:  fmt.Sprintf("Заявка в организацию %s отклонена", org.Name),
			Body:   fmt.Sprintf("Ваша заявка на вступление в организацию %s была отклонена.", org.Name),
			Link:   &link,
			Data: map[string]interface{}{
				"organization_id":    org.ID.String(),
				"organization_name":  org.Name,
				"organization_login": org.Login,
				"request_id":         req.ID.String(),
			},
		})
	}

	return nil
}

func (uc *OrganizationsUseCase) GetMyPendingJoinRequest(ctx context.Context, orgLogin string, userID uuid.UUID) (*models.OrganizationJoinRequest, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}
	return uc.repo.GetPendingJoinRequest(ctx, org.ID, userID)
}
