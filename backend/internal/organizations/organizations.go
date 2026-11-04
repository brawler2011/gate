package organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type OrganizationsUseCase struct {
	repo            OrganizationsRepo
	usersRepo       UsersRepo
	permissionsUC   PermissionsUC
	transactor      OrganizationTransactions
	notificationsUC NotificationsUC
}

func NewOrganizationsUseCase(
	repo OrganizationsRepo,
	usersRepo UsersRepo,
	permissionsUC PermissionsUC,
	transactor OrganizationTransactions,
	notificationsUC NotificationsUC,
) *OrganizationsUseCase {
	return &OrganizationsUseCase{
		repo:            repo,
		usersRepo:       usersRepo,
		permissionsUC:   permissionsUC,
		transactor:      transactor,
		notificationsUC: notificationsUC,
	}
}

func (uc *OrganizationsUseCase) CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error) {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	if accessmodel.UserFromContext(ctx).Id != input.CreatorID {
		return nil, pkg.NoPermission
	}
	var org *organizationsmodel.Organization
	err := uc.transactor.Within(ctx, func(tx OrganizationTxStore) error {
		if err := tx.LockUser(ctx, input.CreatorID); err != nil {
			return err
		}
		creator, err := tx.GetUserById(ctx, input.CreatorID)
		if err != nil {
			return err
		}
		if creator.IsExpired() {
			return pkg.ErrUnauthenticated
		}
		if !creator.IsAdmin() {
			return pkg.NoPermission
		}

		// Create organization
		org, err = tx.CreateOrganization(ctx, input)
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}

		// Add creator as owner
		err = tx.AddMember(ctx, org.ID, input.CreatorID, organizationsmodel.OrgRoleOwner)
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

func (uc *OrganizationsUseCase) GetOrganization(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.Organization, error) {
	if err := accessmodel.RequireCaller(ctx, userID); err != nil {
		return nil, err
	}
	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, orgID, userID, accessmodel.ActionViewOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, pkg.NoPermission
	}

	return uc.repo.GetOrganizationByID(ctx, orgID)
}

func (uc *OrganizationsUseCase) GetOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID) (*organizationsmodel.Organization, error) {
	if err := accessmodel.RequireCaller(ctx, userID); err != nil {
		return nil, err
	}
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return nil, err
	}

	// Check permissions
	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, userID, accessmodel.ActionViewOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, pkg.NoPermission
	}

	return org, nil
}

func (uc *OrganizationsUseCase) ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter, userID uuid.UUID) (*organizationsmodel.OrganizationList, error) {
	if err := accessmodel.RequireCaller(ctx, userID); err != nil {
		return nil, err
	}
	// List all organizations
	orgs, total, err := uc.repo.ListOrganizations(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Filter based on user access
	accessibleOrgs := make([]organizationsmodel.Organization, 0, len(orgs))
	for _, org := range orgs {
		hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, userID, accessmodel.ActionViewOrganization)
		if err != nil {
			return nil, err
		}
		if hasAccess {
			accessibleOrgs = append(accessibleOrgs, org)
		}
	}

	totalPages := int32(0)
	if filter.PageSize > 0 {
		totalPages = (total + filter.PageSize - 1) / filter.PageSize
	}

	return &organizationsmodel.OrganizationList{
		Organizations: accessibleOrgs,
		Pagination: paging.Pagination{
			Page:  filter.Page,
			Total: totalPages,
		},
	}, nil
}

func (uc *OrganizationsUseCase) UpdateOrganization(ctx context.Context, orgID, userID uuid.UUID, input *organizationsmodel.UpdateOrganizationInput) error {
	return uc.withinOrganization(ctx, orgID, userID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error { return tx.UpdateOrganization(ctx, orgID, input) })
}

func (uc *OrganizationsUseCase) UpdateOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID, input *organizationsmodel.UpdateOrganizationInput) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return uc.UpdateOrganization(ctx, org.ID, userID, input)
}

func (uc *OrganizationsUseCase) DeleteOrganization(ctx context.Context, orgID, userID uuid.UUID) error {
	return uc.withinOrganization(ctx, orgID, userID, accessmodel.ActionDeleteOrganization, func(tx OrganizationTxStore) error { return tx.DeleteOrganization(ctx, orgID) })
}

func (uc *OrganizationsUseCase) DeleteOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return uc.DeleteOrganization(ctx, org.ID, userID)
}

func (uc *OrganizationsUseCase) AddMember(ctx context.Context, input *organizationsmodel.AddOrganizationMemberInput, requestUserID uuid.UUID) error {
	return uc.withinOrganization(ctx, input.OrganizationID, requestUserID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error {
		if _, err := tx.GetUserById(ctx, input.UserID); err != nil {
			return err
		}
		return tx.AddMember(ctx, input.OrganizationID, input.UserID, input.Role)
	})
}

func (uc *OrganizationsUseCase) AddMemberByLogin(ctx context.Context, login string, input *organizationsmodel.AddOrganizationMemberInput, requestUserID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	input.OrganizationID = org.ID
	return uc.AddMember(ctx, input, requestUserID)
}

func (uc *OrganizationsUseCase) ListMembers(ctx context.Context, orgID, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationMember, error) {
	if err := accessmodel.RequireCaller(ctx, requestUserID); err != nil {
		return nil, err
	}
	user, err := uc.usersRepo.GetUserById(ctx, requestUserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if user.Role != accessmodel.UserRoleAdmin {
		_, err := uc.repo.GetMember(ctx, orgID, requestUserID)
		if err != nil {
			if errors.Is(err, pkg.ErrNotFound) {
				return nil, pkg.Wrap(pkg.NoPermission, nil, "must be a member of organization")
			}
			return nil, fmt.Errorf("get member: %w", err)
		}
	}

	return uc.repo.ListMembers(ctx, orgID)
}

func (uc *OrganizationsUseCase) ListMembersByLogin(ctx context.Context, login string, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationMember, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	return uc.ListMembers(ctx, org.ID, requestUserID)
}

func (uc *OrganizationsUseCase) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole, requestUserID uuid.UUID) error {
	return uc.withinOrganization(ctx, orgID, requestUserID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error { return tx.UpdateMemberRole(ctx, orgID, userID, role) })
}

func (uc *OrganizationsUseCase) RemoveMember(ctx context.Context, orgID, userID, requestUserID uuid.UUID) error {
	return uc.withinOrganization(ctx, orgID, requestUserID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error { return tx.RemoveMember(ctx, orgID, userID) })
}

func (uc *OrganizationsUseCase) RemoveMemberByLogin(ctx context.Context, login string, userID, requestUserID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return uc.RemoveMember(ctx, org.ID, userID, requestUserID)
}

func (uc *OrganizationsUseCase) ResolveUserOrganizationID(ctx context.Context, userID uuid.UUID, requestedOrgID *uuid.UUID) (uuid.UUID, error) {
	if err := accessmodel.RequireCaller(ctx, userID); err != nil {
		return uuid.Nil, err
	}
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
	input organizationsmodel.BatchCreateOrganizationUsersInput,
	requestUserID uuid.UUID,
) (*organizationsmodel.BatchCreateOrganizationUsersResult, error) {
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

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, requestUserID, accessmodel.ActionManageOrganization)
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
		if err := identitymodel.UsernameValidate(candidate); err != nil {
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
	err = uc.withinOrganization(ctx, org.ID, requestUserID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error {

		for _, pu := range pendingUsers {
			err := tx.CreateUser(ctx, identitymodel.CreateUserParams{
				Id:           pu.id,
				Username:     pu.username,
				Role:         accessmodel.UserRoleUser,
				PasswordHash: pu.passwordHash,
				Email:        nil,
				AvatarUrl:    nil,
				ExpiresAt:    expiresAt,
			})
			if err != nil {
				return fmt.Errorf("create user %s: %w", pu.username, err)
			}

			err = tx.AddMember(ctx, org.ID, pu.id, organizationsmodel.OrgRoleMember)
			if err != nil {
				return fmt.Errorf("add member %s to org: %w", pu.username, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	resultItems := make([]organizationsmodel.BatchCreatedUserItem, len(pendingUsers))
	for i, pu := range pendingUsers {
		resultItems[i] = organizationsmodel.BatchCreatedUserItem{
			ID:        pu.id,
			Username:  pu.username,
			Password:  pu.password,
			ExpiresAt: expiresAt,
		}
	}

	return &organizationsmodel.BatchCreateOrganizationUsersResult{
		Users: resultItems,
	}, nil
}

func (uc *OrganizationsUseCase) InviteMember(
	ctx context.Context,
	orgLogin string,
	targetUserID uuid.UUID,
	role organizationsmodel.OrganizationRole,
	inviterID uuid.UUID,
) (*organizationsmodel.OrganizationInvitation, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}

	var invitation *organizationsmodel.OrganizationInvitation
	var pendingNotifications []notificationsmodel.Notification
	err = uc.withinOrganization(ctx, org.ID, inviterID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error {
		// Verify target user exists
		_, err := tx.GetUserById(ctx, targetUserID)
		if err != nil {
			return pkg.Wrap(pkg.ErrNotFound, err, "user not found")
		}

		inviterUser, err := tx.GetUserById(ctx, inviterID)
		if err != nil {
			return fmt.Errorf("get inviter user: %w", err)
		}

		// Check if already a member
		_, err = tx.GetMember(ctx, org.ID, targetUserID)
		if err == nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, "пользователь уже является участником организации")
		}

		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			return err
		}

		// Check if there is already a pending invitation
		pending, err := tx.GetPendingInvitation(ctx, org.ID, targetUserID)
		if err != nil {
			return fmt.Errorf("get pending invitation: %w", err)
		}
		if pending != nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, "приглашение этому пользователю уже отправлено")
		}

		invitation, err = tx.CreateInvitation(ctx, &organizationsmodel.CreateOrganizationInvitationInput{
			OrganizationID: org.ID,
			UserID:         targetUserID,
			InviterID:      inviterID,
			Role:           role,
		})
		if err != nil {
			return fmt.Errorf("create invitation: %w", err)
		}

		link := "/notifications"
		notification, err := tx.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
			UserID: targetUserID,
			Type:   notificationsmodel.NotificationTypeOrgInvitation,
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
		if err != nil {
			return err
		}
		pendingNotifications = append(pendingNotifications, *notification)

		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, notification := range pendingNotifications {
		if uc.notificationsUC != nil {
			uc.notificationsUC.SendEmail(ctx, notification)
		}
	}

	return invitation, nil
}

func (uc *OrganizationsUseCase) ListInvitations(ctx context.Context, orgLogin string, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationInvitation, error) {
	if err := accessmodel.RequireCaller(ctx, requestUserID); err != nil {
		return nil, err
	}
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, requestUserID, accessmodel.ActionManageOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	status := string(organizationsmodel.RequestStatusPending)
	return uc.repo.ListInvitations(ctx, org.ID, &status)
}

func (uc *OrganizationsUseCase) CancelInvitation(ctx context.Context, orgLogin string, invitationID, requestUserID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	return uc.withinOrganization(ctx, org.ID, requestUserID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error {
		inv, err := tx.GetInvitationByID(ctx, invitationID)
		if err != nil {
			return err
		}
		if inv.OrganizationID != org.ID {
			return pkg.ErrBadInput
		}
		return tx.UpdateInvitationStatus(ctx, invitationID, organizationsmodel.RequestStatusCanceled)
	})
}

func (uc *OrganizationsUseCase) AcceptInvitation(ctx context.Context, invitationID uuid.UUID) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return pkg.ErrUnauthenticated
	}
	inv, err := uc.repo.GetInvitationByID(ctx, invitationID)
	if err != nil {
		return err
	}
	return uc.transactor.Within(ctx, func(tx OrganizationTxStore) error {
		if err := tx.LockOrganization(ctx, inv.OrganizationID); err != nil {
			return err
		}
		inv, err := tx.GetInvitationByID(ctx, invitationID)
		if err != nil {
			return err
		}
		if inv.UserID != user.Id {
			return pkg.NoPermission
		}
		if inv.Status != organizationsmodel.RequestStatusPending {
			return pkg.ErrConflict
		}
		if err := tx.AddMember(ctx, inv.OrganizationID, inv.UserID, inv.Role); err != nil {
			return err
		}
		return tx.UpdateInvitationStatus(ctx, inv.ID, organizationsmodel.RequestStatusAccepted)
	})
}

func (uc *OrganizationsUseCase) DeclineInvitation(ctx context.Context, invitationID uuid.UUID) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return pkg.ErrUnauthenticated
	}
	inv, err := uc.repo.GetInvitationByID(ctx, invitationID)
	if err != nil {
		return err
	}
	return uc.transactor.Within(ctx, func(tx OrganizationTxStore) error {
		if err := tx.LockOrganization(ctx, inv.OrganizationID); err != nil {
			return err
		}
		inv, err := tx.GetInvitationByID(ctx, invitationID)
		if err != nil {
			return err
		}
		if inv.UserID != user.Id {
			return pkg.NoPermission
		}
		if inv.Status != organizationsmodel.RequestStatusPending {
			return pkg.ErrConflict
		}
		return tx.UpdateInvitationStatus(ctx, inv.ID, organizationsmodel.RequestStatusDeclined)
	})
}

func (uc *OrganizationsUseCase) CreateJoinRequest(
	ctx context.Context,
	orgLogin string,
	message *string,
) (*organizationsmodel.OrganizationJoinRequest, bool, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, false, err
	}

	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return nil, false, pkg.ErrUnauthenticated
	}
	userID := user.Id
	var joined bool

	var req *organizationsmodel.OrganizationJoinRequest
	var pendingNotifications []notificationsmodel.Notification
	err = uc.transactor.Within(ctx, func(tx OrganizationTxStore) error {
		if err := tx.LockOrganization(ctx, org.ID); err != nil {
			return err
		}
		org, err := tx.GetOrganizationByID(ctx, org.ID)
		if err != nil {
			return err
		}

		// Check if already a member
		_, err = tx.GetMember(ctx, org.ID, userID)
		if err == nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, "вы уже состоите в этой организации")
		}

		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			return err
		}

		if org.JoinPolicy == organizationsmodel.OrgJoinPolicyInviteOnly {
			return pkg.Wrap(pkg.ErrBadInput, nil, "вступление в организацию только по приглашениям")
		}

		if org.JoinPolicy == organizationsmodel.OrgJoinPolicyOpen {
			// Immediately add member
			if err := tx.AddMember(ctx, org.ID, userID, organizationsmodel.OrgRoleMember); err != nil {
				return fmt.Errorf("add member: %w", err)
			}
			joined = true
			return nil
		}

		// By Request
		pending, err := tx.GetPendingJoinRequest(ctx, org.ID, userID)
		if err != nil {
			return fmt.Errorf("get pending request: %w", err)
		}
		if pending != nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, "заявка на вступление уже подана и ожидает рассмотрения")
		}

		req, err = tx.CreateJoinRequest(ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{
			OrganizationID: org.ID,
			UserID:         userID,
			Message:        message,
		})
		if err != nil {
			return fmt.Errorf("create join request: %w", err)
		}

		members, err := tx.ListMembers(ctx, org.ID)
		if err != nil {
			return err
		}

		link := fmt.Sprintf("/%s/settings/members", org.Login)
		for _, m := range members {
			if m.Role == organizationsmodel.OrgRoleOwner || m.Role == organizationsmodel.OrgRoleAdmin {
				notification, err := tx.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
					UserID: m.UserID,
					Type:   notificationsmodel.NotificationTypeOrgJoinRequest,
					Title:  fmt.Sprintf("Новая заявка в организацию %s", org.Name),
					Body:   fmt.Sprintf("Пользователь @%s подал заявку на вступление в организацию %s", user.Username, org.Name),
					Link:   &link,
					Data: map[string]interface{}{
						"organization_id":    org.ID.String(),
						"organization_name":  org.Name,
						"organization_login": org.Login,
						"applicant_id":       userID.String(),
						"applicant_username": user.Username,
						"request_id":         req.ID.String(),
					},
				})
				if err != nil {
					return err
				}
				pendingNotifications = append(pendingNotifications, *notification)
			}
		}

		return nil
	})
	if err != nil {
		return nil, false, err
	}

	for _, notification := range pendingNotifications {
		if uc.notificationsUC != nil {
			uc.notificationsUC.SendEmail(ctx, notification)
		}
	}

	return req, joined, nil
}

func (uc *OrganizationsUseCase) ListJoinRequests(ctx context.Context, orgLogin string, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationJoinRequest, error) {
	if err := accessmodel.RequireCaller(ctx, requestUserID); err != nil {
		return nil, err
	}
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}

	hasAccess, err := uc.permissionsUC.HasOrganizationPermission(ctx, org.ID, requestUserID, accessmodel.ActionManageOrganization)
	if err != nil {
		return nil, fmt.Errorf("check permissions: %w", err)
	}
	if !hasAccess {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	status := string(organizationsmodel.RequestStatusPending)
	return uc.repo.ListJoinRequests(ctx, org.ID, &status)
}

func (uc *OrganizationsUseCase) CancelJoinRequest(ctx context.Context, orgLogin string) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() || user.IsExpired() {
		return pkg.ErrUnauthenticated
	}
	return uc.transactor.Within(ctx, func(tx OrganizationTxStore) error {
		if err := tx.LockOrganization(ctx, org.ID); err != nil {
			return err
		}
		req, err := tx.GetPendingJoinRequest(ctx, org.ID, user.Id)
		if err != nil {
			return err
		}
		if req == nil {
			return pkg.ErrNotFound
		}
		return tx.UpdateJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusCanceled, nil)
	})
}

func (uc *OrganizationsUseCase) ApproveJoinRequest(
	ctx context.Context,
	orgLogin string,
	requestID, reviewerID uuid.UUID,
	role organizationsmodel.OrganizationRole,
) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	var pendingNotifications []notificationsmodel.Notification
	err = uc.withinOrganization(ctx, org.ID, reviewerID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error {
		req, err := tx.GetJoinRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if req.OrganizationID != org.ID {
			return pkg.Wrap(pkg.ErrBadInput, nil, "request does not belong to this organization")
		}
		if req.Status != organizationsmodel.RequestStatusPending {
			return pkg.Wrap(pkg.ErrBadInput, nil, "заявка уже рассмотрена")
		}

		if err := tx.AddMember(ctx, req.OrganizationID, req.UserID, role); err != nil {
			return fmt.Errorf("add member: %w", err)
		}

		if err := tx.UpdateJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusApproved, &reviewerID); err != nil {
			return fmt.Errorf("update join request status: %w", err)
		}

		link := fmt.Sprintf("/%s", org.Login)
		notification, err := tx.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
			UserID: req.UserID,
			Type:   notificationsmodel.NotificationTypeOrgJoinApproved,
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
		if err != nil {
			return err
		}
		pendingNotifications = append(pendingNotifications, *notification)

		return nil
	})
	if err != nil {
		return err
	}

	for _, notification := range pendingNotifications {
		if uc.notificationsUC != nil {
			uc.notificationsUC.SendEmail(ctx, notification)
		}
	}

	return nil
}

func (uc *OrganizationsUseCase) RejectJoinRequest(ctx context.Context, orgLogin string, requestID, reviewerID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	var pendingNotifications []notificationsmodel.Notification
	err = uc.withinOrganization(ctx, org.ID, reviewerID, accessmodel.ActionManageOrganization, func(tx OrganizationTxStore) error {
		req, err := tx.GetJoinRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if req.OrganizationID != org.ID {
			return pkg.Wrap(pkg.ErrBadInput, nil, "request does not belong to this organization")
		}
		if req.Status != organizationsmodel.RequestStatusPending {
			return pkg.Wrap(pkg.ErrBadInput, nil, "заявка уже рассмотрена")
		}

		if err := tx.UpdateJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusRejected, &reviewerID); err != nil {
			return fmt.Errorf("update join request status: %w", err)
		}

		link := fmt.Sprintf("/%s", org.Login)
		notification, err := tx.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
			UserID: req.UserID,
			Type:   notificationsmodel.NotificationTypeOrgJoinRejected,
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
		if err != nil {
			return err
		}
		pendingNotifications = append(pendingNotifications, *notification)

		return nil
	})
	if err != nil {
		return err
	}

	for _, notification := range pendingNotifications {
		if uc.notificationsUC != nil {
			uc.notificationsUC.SendEmail(ctx, notification)
		}
	}

	return nil
}

func (uc *OrganizationsUseCase) GetMyPendingJoinRequest(ctx context.Context, orgLogin string, userID uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error) {
	if err := accessmodel.RequireCaller(ctx, userID); err != nil {
		return nil, err
	}
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}
	return uc.repo.GetPendingJoinRequest(ctx, org.ID, userID)
}

func (uc *OrganizationsUseCase) withinOrganization(ctx context.Context, id, actor uuid.UUID, action accessmodel.OrgAction, fn func(OrganizationTxStore) error) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	if user.Id != actor {
		return pkg.NoPermission
	}
	return uc.transactor.Within(ctx, func(tx OrganizationTxStore) error {
		if err := tx.LockOrganization(ctx, id); err != nil {
			return err
		}
		allowed, err := tx.HasOrganizationPermission(ctx, id, actor, action)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.NoPermission
		}
		return fn(tx)
	})
}
