package notifications

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/google/uuid"
)

type NotificationsRepo interface {
	CreateNotification(ctx context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error)

	MarkAsRead(ctx context.Context, id, userID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
}

type UsersRepo interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
}
