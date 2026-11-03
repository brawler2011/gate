package notifications

import (
	"context"
	"fmt"
	"log/slog"

	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/google/uuid"
)

type NotificationsUseCase struct {
	repo         NotificationsRepo
	usersRepo    UsersRepo
	emailService email.EmailService
}

func NewNotificationsUseCase(
	repo NotificationsRepo,
	usersRepo UsersRepo,
	emailService email.EmailService,
) *NotificationsUseCase {
	return &NotificationsUseCase{
		repo:         repo,
		usersRepo:    usersRepo,
		emailService: emailService,
	}
}

func (uc *NotificationsUseCase) Notify(ctx context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	notif, err := uc.repo.CreateNotification(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create notification in repo: %w", err)
	}

	uc.sendEmail(ctx, input.UserID, *notif)

	return notif, nil
}

func (uc *NotificationsUseCase) sendEmail(ctx context.Context, targetUserID uuid.UUID, n notificationsmodel.Notification) {
	user, err := uc.usersRepo.GetUserById(ctx, targetUserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load notification email recipient", "error", err, "user_id", targetUserID)
		return
	}
	if user.Email == nil || *user.Email == "" {
		return
	}

	userEmail := *user.Email
	username := user.Username

	switch n.Type {
	case notificationsmodel.NotificationTypeOrgInvitation:
		orgName, _ := n.Data["organization_name"].(string)
		orgLogin, _ := n.Data["organization_login"].(string)
		role, _ := n.Data["role"].(string)
		inviter, _ := n.Data["inviter_username"].(string)
		if err := uc.emailService.SendOrgInvitationEmail(ctx, userEmail, username, orgName, orgLogin, role, inviter); err != nil {
			slog.Error("failed to send org invitation email", "error", err, "user_id", targetUserID)
		}

	case notificationsmodel.NotificationTypeOrgJoinRequest:
		orgName, _ := n.Data["organization_name"].(string)
		orgLogin, _ := n.Data["organization_login"].(string)
		applicant, _ := n.Data["applicant_username"].(string)
		if err := uc.emailService.SendOrgJoinRequestEmail(ctx, userEmail, username, applicant, orgName, orgLogin); err != nil {
			slog.Error("failed to send org join request email", "error", err, "user_id", targetUserID)
		}

	case notificationsmodel.NotificationTypeOrgJoinApproved:
		orgName, _ := n.Data["organization_name"].(string)
		orgLogin, _ := n.Data["organization_login"].(string)
		if err := uc.emailService.SendOrgJoinRequestResolvedEmail(ctx, userEmail, username, orgName, orgLogin, true); err != nil {
			slog.Error("failed to send org join approved email", "error", err, "user_id", targetUserID)
		}

	case notificationsmodel.NotificationTypeOrgJoinRejected:
		orgName, _ := n.Data["organization_name"].(string)
		orgLogin, _ := n.Data["organization_login"].(string)
		if err := uc.emailService.SendOrgJoinRequestResolvedEmail(ctx, userEmail, username, orgName, orgLogin, false); err != nil {
			slog.Error("failed to send org join rejected email", "error", err, "user_id", targetUserID)
		}

	case notificationsmodel.NotificationTypeContestJoinRequest:
		contestTitle, _ := n.Data["contest_title"].(string)
		orgLogin, _ := n.Data["organization_login"].(string)
		contestLogin, _ := n.Data["contest_login"].(string)
		applicant, _ := n.Data["applicant_username"].(string)
		if err := uc.emailService.SendContestJoinRequestEmail(ctx, userEmail, username, applicant, contestTitle, orgLogin, contestLogin); err != nil {
			slog.Error("failed to send contest join request email", "error", err, "user_id", targetUserID)
		}

	case notificationsmodel.NotificationTypeContestJoinApproved:
		contestTitle, _ := n.Data["contest_title"].(string)
		orgLogin, _ := n.Data["organization_login"].(string)
		contestLogin, _ := n.Data["contest_login"].(string)
		if err := uc.emailService.SendContestJoinRequestResolvedEmail(ctx, userEmail, username, contestTitle, orgLogin, contestLogin, true); err != nil {
			slog.Error("failed to send contest join approved email", "error", err, "user_id", targetUserID)
		}

	case notificationsmodel.NotificationTypeContestJoinRejected:
		contestTitle, _ := n.Data["contest_title"].(string)
		orgLogin, _ := n.Data["organization_login"].(string)
		contestLogin, _ := n.Data["contest_login"].(string)
		if err := uc.emailService.SendContestJoinRequestResolvedEmail(ctx, userEmail, username, contestTitle, orgLogin, contestLogin, false); err != nil {
			slog.Error("failed to send contest join rejected email", "error", err, "user_id", targetUserID)
		}
	}
}

func (uc *NotificationsUseCase) MarkAsRead(ctx context.Context, id, userID uuid.UUID) error {
	return uc.repo.MarkAsRead(ctx, id, userID)
}

func (uc *NotificationsUseCase) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	return uc.repo.MarkAllAsRead(ctx, userID)
}
