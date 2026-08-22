package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type NotificationsRepo interface {
	CreateNotification(ctx context.Context, input *models.CreateNotificationInput) (*models.Notification, error)
	ListNotifications(ctx context.Context, userID uuid.UUID, filter *models.NotificationFilter) (*models.NotificationsList, error)
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int32, error)
	GetNotificationByID(ctx context.Context, id, userID uuid.UUID) (*models.Notification, error)
	MarkAsRead(ctx context.Context, id, userID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
	WithTx(tx pgx.Tx) NotificationsRepo
}

type NotificationsUC interface {
	Notify(ctx context.Context, input *models.CreateNotificationInput) (*models.Notification, error)
	ListNotifications(ctx context.Context, userID uuid.UUID, filter *models.NotificationFilter) (*models.NotificationsList, error)
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int32, error)
	MarkAsRead(ctx context.Context, id, userID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
}
