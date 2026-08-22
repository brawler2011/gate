package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/google/uuid"
)

type NotificationsUseCase struct {
	repo         interfaces.NotificationsRepo
	usersRepo    interfaces.UsersRepo
	emailService email.EmailService
}

func NewNotificationsUseCase(
	repo interfaces.NotificationsRepo,
	usersRepo interfaces.UsersRepo,
	emailService email.EmailService,
) *NotificationsUseCase {
	return &NotificationsUseCase{
		repo:         repo,
		usersRepo:    usersRepo,
		emailService: emailService,
	}
}

func (uc *NotificationsUseCase) Notify(ctx context.Context, input *models.CreateNotificationInput) (*models.Notification, error) {
	notif, err := uc.repo.CreateNotification(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create notification in repo: %w", err)
	}

	// Fail-safe asynchronous email delivery
	//nolint:gosec // async email notification is intentionally detached from the request context
	go func(targetUserID uuid.UUID, n models.Notification) {
		bgCtx := context.Background()
		user, err := uc.usersRepo.GetUserById(bgCtx, targetUserID)
		if err != nil || user.Email == nil || *user.Email == "" {
			return
		}

		userEmail := *user.Email
		username := user.Username

		switch n.Type {
		case models.NotificationTypeOrgInvitation:
			orgName, _ := n.Data["organization_name"].(string)
			orgLogin, _ := n.Data["organization_login"].(string)
			role, _ := n.Data["role"].(string)
			inviter, _ := n.Data["inviter_username"].(string)
			if err := uc.emailService.SendOrgInvitationEmail(bgCtx, userEmail, username, orgName, orgLogin, role, inviter); err != nil {
				slog.Error("failed to send org invitation email", "error", err, "user_id", targetUserID)
			}

		case models.NotificationTypeOrgJoinRequest:
			orgName, _ := n.Data["organization_name"].(string)
			orgLogin, _ := n.Data["organization_login"].(string)
			applicant, _ := n.Data["applicant_username"].(string)
			if err := uc.emailService.SendOrgJoinRequestEmail(bgCtx, userEmail, username, applicant, orgName, orgLogin); err != nil {
				slog.Error("failed to send org join request email", "error", err, "user_id", targetUserID)
			}

		case models.NotificationTypeOrgJoinApproved:
			orgName, _ := n.Data["organization_name"].(string)
			orgLogin, _ := n.Data["organization_login"].(string)
			if err := uc.emailService.SendOrgJoinRequestResolvedEmail(bgCtx, userEmail, username, orgName, orgLogin, true); err != nil {
				slog.Error("failed to send org join approved email", "error", err, "user_id", targetUserID)
			}

		case models.NotificationTypeOrgJoinRejected:
			orgName, _ := n.Data["organization_name"].(string)
			orgLogin, _ := n.Data["organization_login"].(string)
			if err := uc.emailService.SendOrgJoinRequestResolvedEmail(bgCtx, userEmail, username, orgName, orgLogin, false); err != nil {
				slog.Error("failed to send org join rejected email", "error", err, "user_id", targetUserID)
			}

		case models.NotificationTypeContestJoinRequest:
			contestTitle, _ := n.Data["contest_title"].(string)
			orgLogin, _ := n.Data["organization_login"].(string)
			contestLogin, _ := n.Data["contest_login"].(string)
			applicant, _ := n.Data["applicant_username"].(string)
			if err := uc.emailService.SendContestJoinRequestEmail(bgCtx, userEmail, username, applicant, contestTitle, orgLogin, contestLogin); err != nil {
				slog.Error("failed to send contest join request email", "error", err, "user_id", targetUserID)
			}

		case models.NotificationTypeContestJoinApproved:
			contestTitle, _ := n.Data["contest_title"].(string)
			orgLogin, _ := n.Data["organization_login"].(string)
			contestLogin, _ := n.Data["contest_login"].(string)
			if err := uc.emailService.SendContestJoinRequestResolvedEmail(bgCtx, userEmail, username, contestTitle, orgLogin, contestLogin, true); err != nil {
				slog.Error("failed to send contest join approved email", "error", err, "user_id", targetUserID)
			}

		case models.NotificationTypeContestJoinRejected:
			contestTitle, _ := n.Data["contest_title"].(string)
			orgLogin, _ := n.Data["organization_login"].(string)
			contestLogin, _ := n.Data["contest_login"].(string)
			if err := uc.emailService.SendContestJoinRequestResolvedEmail(bgCtx, userEmail, username, contestTitle, orgLogin, contestLogin, false); err != nil {
				slog.Error("failed to send contest join rejected email", "error", err, "user_id", targetUserID)
			}
		}
	}(input.UserID, *notif)

	return notif, nil
}

func (uc *NotificationsUseCase) ListNotifications(ctx context.Context, userID uuid.UUID, filter *models.NotificationFilter) (*models.NotificationsList, error) {
	if filter == nil {
		filter = &models.NotificationFilter{
			Page:     1,
			PageSize: 20,
		}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return uc.repo.ListNotifications(ctx, userID, filter)
}

func (uc *NotificationsUseCase) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int32, error) {
	return uc.repo.GetUnreadCount(ctx, userID)
}

func (uc *NotificationsUseCase) MarkAsRead(ctx context.Context, id, userID uuid.UUID) error {
	return uc.repo.MarkAsRead(ctx, id, userID)
}

func (uc *NotificationsUseCase) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	return uc.repo.MarkAllAsRead(ctx, userID)
}
