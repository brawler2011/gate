package organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
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
	access          access.Loader
	transactor      OrganizationTransactions
	notificationsUC NotificationsUC
}

func NewOrganizationsUseCase(
	repo OrganizationsRepo,
	usersRepo UsersRepo,
	loader access.Loader,
	transactor OrganizationTransactions,
	notificationsUC NotificationsUC,
) *OrganizationsUseCase {
	return &OrganizationsUseCase{
		repo:            repo,
		usersRepo:       usersRepo,
		access:          loader,
		transactor:      transactor,
		notificationsUC: notificationsUC,
	}
}

func (uc *OrganizationsUseCase) CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error) {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	var org *organizationsmodel.Organization
	err := access.WithinActor(ctx, uc.transactor, func(tx OrganizationTxStore, creator accessmodel.User) error {
		if !creator.IsAdmin() {
			return pkg.ErrForbidden
		}

		var err error
		org, err = tx.CreateOrganization(ctx, input)
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}

		err = tx.AddMember(ctx, org.ID, creator.Id, accessmodel.OrganizationOwner)
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

func (uc *OrganizationsUseCase) GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if _, err := access.Organization(ctx, uc.access, org.ID, accessmodel.OrganizationView); err != nil {
		return nil, err
	}
	return org, nil
}

func (uc *OrganizationsUseCase) ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter) (*organizationsmodel.OrganizationList, error) {
	if filter == nil {
		return nil, pkg.ErrInvalid
	}
	normalized := *filter
	var err error
	normalized.Page, normalized.PageSize, err = paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	filter = &normalized
	orgs, total, err := uc.repo.ListOrganizations(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &organizationsmodel.OrganizationList{
		Organizations: orgs,
		Pagination:    paging.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *OrganizationsUseCase) UpdateOrganizationByLogin(ctx context.Context, login string, input *organizationsmodel.UpdateOrganizationInput) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return access.WithinOrganization(ctx, uc.transactor, org.ID, accessmodel.OrganizationManage, func(tx OrganizationTxStore, _ access.OrganizationAccess) error {
		return tx.UpdateOrganization(ctx, org.ID, input)
	})
}

func (uc *OrganizationsUseCase) DeleteOrganizationByLogin(ctx context.Context, login string) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return access.WithinOrganization(ctx, uc.transactor, org.ID, accessmodel.OrganizationDelete, func(tx OrganizationTxStore, _ access.OrganizationAccess) error {
		return tx.DeleteOrganization(ctx, org.ID)
	})
}

// AddMember skips the user's consent, so only site administrators may use it.
func (uc *OrganizationsUseCase) AddMember(ctx context.Context, input *organizationsmodel.AddOrganizationMemberInput) error {
	return access.WithinOrganization(ctx, uc.transactor, input.OrganizationID, accessmodel.OrganizationManageMembers, func(tx OrganizationTxStore, a access.OrganizationAccess) error {
		if !a.Actor.IsAdmin() {
			return pkg.ErrForbidden
		}
		if _, err := tx.GetUserById(ctx, input.UserID); err != nil {
			return err
		}
		return tx.AddMember(ctx, input.OrganizationID, input.UserID, input.Role)
	})
}

func (uc *OrganizationsUseCase) AddMemberByLogin(ctx context.Context, login string, input *organizationsmodel.AddOrganizationMemberInput) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	input.OrganizationID = org.ID
	return uc.AddMember(ctx, input)
}

func (uc *OrganizationsUseCase) ListMembersByLogin(ctx context.Context, login string, request paging.Request) (*paging.List[organizationsmodel.OrganizationMember], error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	orgID := org.ID
	if _, err := access.Organization(ctx, uc.access, orgID, accessmodel.OrganizationListMembers); err != nil {
		return nil, err
	}
	request, err = request.Normalize()
	if err != nil {
		return nil, err
	}
	items, total, err := uc.repo.ListMembersPage(ctx, orgID, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[organizationsmodel.OrganizationMember]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, total)}, nil
}

func (uc *OrganizationsUseCase) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role accessmodel.OrganizationRole) error {
	return uc.changeMember(ctx, orgID, userID, role, func(tx OrganizationTxStore) error { return tx.UpdateMemberRole(ctx, orgID, userID, role) })
}

func (uc *OrganizationsUseCase) RemoveMemberByLogin(ctx context.Context, login string, userID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return err
	}
	return uc.changeMember(ctx, org.ID, userID, "", func(tx OrganizationTxStore) error { return tx.RemoveMember(ctx, org.ID, userID) })
}

// Members may lower or drop their own role without managing members.
func (uc *OrganizationsUseCase) changeMember(ctx context.Context, orgID, userID uuid.UUID, role accessmodel.OrganizationRole, write func(OrganizationTxStore) error) error {
	return access.WithinOrganization(ctx, uc.transactor, orgID, accessmodel.OrganizationView, func(tx OrganizationTxStore, a access.OrganizationAccess) error {
		member, err := tx.GetMember(ctx, orgID, userID)
		if err != nil {
			return err
		}
		if err := a.ChangeMember(userID, member.Role, role); err != nil {
			return err
		}
		return write(tx)
	})
}

func (uc *OrganizationsUseCase) BatchCreateUsers(
	ctx context.Context,
	input organizationsmodel.BatchCreateOrganizationUsersInput,
) (*organizationsmodel.BatchCreateOrganizationUsersResult, error) {
	if input.Count < 1 || input.Count > 500 {
		return nil, pkg.AtField("/count", pkg.Invalid(pkg.ErrRange, "count must be between 1 and 500", map[string]any{"min": 1, "max": 500}))
	}

	prefix := strings.TrimSpace(input.Prefix)
	if prefix == "" {
		return nil, pkg.AtField("/prefix", pkg.Invalid(pkg.ErrRequired, "prefix cannot be empty", nil))
	}
	if len(prefix) > 20 {
		return nil, pkg.AtField("/prefix", pkg.Invalid(pkg.ErrLength, "prefix cannot exceed 20 characters", map[string]any{"max": 20}))
	}

	org, err := uc.repo.GetOrganizationByLogin(ctx, input.OrgLogin)
	if err != nil {
		return nil, err
	}

	// Fail before hashing passwords; the transaction checks again.
	if _, err := access.Organization(ctx, uc.access, org.ID, accessmodel.OrganizationManageMembers); err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if input.TTLDays != nil && *input.TTLDays > 0 {
		t := time.Now().Add(time.Duration(*input.TTLDays) * 24 * time.Hour)
		expiresAt = &t
	}

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
			return nil, pkg.AtField("/prefix", err)
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

	err = access.WithinOrganization(ctx, uc.transactor, org.ID, accessmodel.OrganizationManageMembers, func(tx OrganizationTxStore, _ access.OrganizationAccess) error {
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

			err = tx.AddMember(ctx, org.ID, pu.id, accessmodel.OrganizationMember)
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
	role accessmodel.OrganizationRole,
) (*organizationsmodel.OrganizationInvitation, error) {
	inviterID := accessmodel.UserFromContext(ctx).Id
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}

	var invitation *organizationsmodel.OrganizationInvitation
	var pendingNotifications []notificationsmodel.Notification
	err = access.WithinOrganization(ctx, uc.transactor, org.ID, accessmodel.OrganizationInvite, func(tx OrganizationTxStore, a access.OrganizationAccess) error {
		if err := a.Invite(targetUserID, role); err != nil {
			return err
		}
		_, err := tx.GetUserById(ctx, targetUserID)
		if err != nil {
			return pkg.OnNotFound(err, pkg.ErrNotFound, "user not found")
		}

		inviterUser, err := tx.GetUserById(ctx, inviterID)
		if err != nil {
			return fmt.Errorf("get inviter user: %w", err)
		}

		_, err = tx.GetMember(ctx, org.ID, targetUserID)
		if err == nil {
			return pkg.Wrap(pkg.ErrAlreadyMember, nil, "user is already a member of the organization")
		}

		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			return err
		}

		pending, err := tx.GetPendingInvitation(ctx, org.ID, targetUserID)
		if err != nil {
			return fmt.Errorf("get pending invitation: %w", err)
		}
		if pending != nil {
			return pkg.Wrap(pkg.ErrInvitationPending, nil, "an invitation to this user is already pending")
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

func (uc *OrganizationsUseCase) ListInvitations(ctx context.Context, orgLogin string) ([]organizationsmodel.OrganizationInvitation, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}
	if _, err := access.Organization(ctx, uc.access, org.ID, accessmodel.OrganizationInvite); err != nil {
		return nil, err
	}

	status := string(organizationsmodel.RequestStatusPending)
	return uc.repo.ListInvitations(ctx, org.ID, &status)
}

func (uc *OrganizationsUseCase) CancelInvitation(ctx context.Context, orgLogin string, invitationID uuid.UUID) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	return access.WithinOrganization(ctx, uc.transactor, org.ID, accessmodel.OrganizationInvite, func(tx OrganizationTxStore, a access.OrganizationAccess) error {
		inv, err := tx.GetInvitationByID(ctx, invitationID)
		if err != nil {
			return err
		}
		if inv.OrganizationID != org.ID {
			return pkg.ErrNotFound
		}
		if err := a.Invite(inv.UserID, inv.Role); err != nil {
			return err
		}
		return tx.UpdateInvitationStatus(ctx, invitationID, organizationsmodel.RequestStatusCanceled)
	})
}

func (uc *OrganizationsUseCase) AcceptInvitation(ctx context.Context, invitationID uuid.UUID) error {
	return access.WithinActor(ctx, uc.transactor, func(tx OrganizationTxStore, user accessmodel.User) error {
		inv, err := tx.GetInvitationByID(ctx, invitationID)
		if err != nil {
			return err
		}
		if inv.UserID != user.Id {
			return pkg.ErrForbidden
		}
		if inv.Status != organizationsmodel.RequestStatusPending {
			return pkg.ErrRequestResolved
		}
		if err := tx.UpdateInvitationStatus(ctx, inv.ID, organizationsmodel.RequestStatusAccepted); err != nil {
			return err
		}
		return tx.AddMember(ctx, inv.OrganizationID, inv.UserID, inv.Role)
	})
}

func (uc *OrganizationsUseCase) DeclineInvitation(ctx context.Context, invitationID uuid.UUID) error {
	return access.WithinActor(ctx, uc.transactor, func(tx OrganizationTxStore, user accessmodel.User) error {
		inv, err := tx.GetInvitationByID(ctx, invitationID)
		if err != nil {
			return err
		}
		if inv.UserID != user.Id {
			return pkg.ErrForbidden
		}
		if inv.Status != organizationsmodel.RequestStatusPending {
			return pkg.ErrRequestResolved
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

	var joined bool

	var req *organizationsmodel.OrganizationJoinRequest
	var pendingNotifications []notificationsmodel.Notification
	err = access.WithinActor(ctx, uc.transactor, func(tx OrganizationTxStore, user accessmodel.User) error {
		userID := user.Id
		org, err := tx.GetOrganizationByID(ctx, org.ID)
		if err != nil {
			return err
		}

		_, err = tx.GetMember(ctx, org.ID, userID)
		if err == nil {
			return pkg.Wrap(pkg.ErrAlreadyMember, nil, "already a member of the organization")
		}

		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			return err
		}

		if org.JoinPolicy == accessmodel.JoinInviteOnly {
			return pkg.Wrap(pkg.ErrInviteOnly, nil, "the organization accepts members by invitation only")
		}

		if org.JoinPolicy == accessmodel.JoinOpen {
			if err := tx.AddMember(ctx, org.ID, userID, accessmodel.OrganizationMember); err != nil {
				return fmt.Errorf("add member: %w", err)
			}
			joined = true
			return nil
		}

		pending, err := tx.GetPendingJoinRequest(ctx, org.ID, userID)
		if err != nil {
			return fmt.Errorf("get pending request: %w", err)
		}
		if pending != nil {
			return pkg.Wrap(pkg.ErrRequestPending, nil, "a join request is already pending")
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
			if m.Role == accessmodel.OrganizationOwner || m.Role == accessmodel.OrganizationAdmin {
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

func (uc *OrganizationsUseCase) ListJoinRequests(ctx context.Context, orgLogin string) ([]organizationsmodel.OrganizationJoinRequest, error) {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}
	if _, err := access.Organization(ctx, uc.access, org.ID, accessmodel.OrganizationManageMembers); err != nil {
		return nil, err
	}

	status := string(organizationsmodel.RequestStatusPending)
	return uc.repo.ListJoinRequests(ctx, org.ID, &status)
}

func (uc *OrganizationsUseCase) CancelJoinRequest(ctx context.Context, orgLogin string) error {
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	return access.WithinActor(ctx, uc.transactor, func(tx OrganizationTxStore, user accessmodel.User) error {
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
	requestID uuid.UUID,
	role accessmodel.OrganizationRole,
) error {
	reviewerID := accessmodel.UserFromContext(ctx).Id
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	var pendingNotifications []notificationsmodel.Notification
	err = access.WithinOrganization(ctx, uc.transactor, org.ID, accessmodel.OrganizationManageMembers, func(tx OrganizationTxStore, a access.OrganizationAccess) error {
		req, err := tx.GetJoinRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if req.OrganizationID != org.ID {
			return pkg.Wrap(pkg.ErrNotFound, nil, "request does not belong to this organization")
		}
		if req.Status != organizationsmodel.RequestStatusPending {
			return pkg.Wrap(pkg.ErrRequestResolved, nil, "the request is already resolved")
		}
		if err := a.ChangeMember(req.UserID, "", role); err != nil {
			return err
		}

		if err := tx.UpdateJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusApproved, &reviewerID); err != nil {
			return fmt.Errorf("update join request status: %w", err)
		}

		if err := tx.AddMember(ctx, req.OrganizationID, req.UserID, role); err != nil {
			return fmt.Errorf("add member: %w", err)
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

func (uc *OrganizationsUseCase) RejectJoinRequest(ctx context.Context, orgLogin string, requestID uuid.UUID) error {
	reviewerID := accessmodel.UserFromContext(ctx).Id
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return err
	}

	var pendingNotifications []notificationsmodel.Notification
	err = access.WithinOrganization(ctx, uc.transactor, org.ID, accessmodel.OrganizationManageMembers, func(tx OrganizationTxStore, _ access.OrganizationAccess) error {
		req, err := tx.GetJoinRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if req.OrganizationID != org.ID {
			return pkg.Wrap(pkg.ErrNotFound, nil, "request does not belong to this organization")
		}
		if req.Status != organizationsmodel.RequestStatusPending {
			return pkg.Wrap(pkg.ErrRequestResolved, nil, "the request is already resolved")
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

func (uc *OrganizationsUseCase) GetMyPendingJoinRequest(ctx context.Context, orgLogin string) (*organizationsmodel.OrganizationJoinRequest, error) {
	user := accessmodel.UserFromContext(ctx)
	org, err := uc.repo.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, err
	}
	return uc.repo.GetPendingJoinRequest(ctx, org.ID, user.Id)
}
