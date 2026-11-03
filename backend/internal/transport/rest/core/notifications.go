package core

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type NotificationsReader interface {
	ListNotifications(ctx context.Context, userID uuid.UUID, filter *notificationsmodel.NotificationFilter) (*notificationsmodel.NotificationsList, error)
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int32, error)
}

func (h *CoreServer) ListNotifications(ctx context.Context, params corev1.ListNotificationsParams) (*corev1.NotificationsListResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(20)
	unreadOnly := params.UnreadOnly.Or(false)

	list, err := h.notificationsReader.ListNotifications(ctx, user.Id, &notificationsmodel.NotificationFilter{
		Page:       page,
		PageSize:   pageSize,
		UnreadOnly: unreadOnly,
	})
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list notifications")
	}

	return NotificationsListResponseDTO(list), nil
}

func (h *CoreServer) GetUnreadNotificationsCount(ctx context.Context) (*corev1.UnreadNotificationsCountResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return &corev1.UnreadNotificationsCountResponseModel{Count: 0}, nil
	}

	count, err := h.notificationsReader.GetUnreadCount(ctx, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get unread notifications count")
	}

	return &corev1.UnreadNotificationsCountResponseModel{Count: count}, nil
}

func (h *CoreServer) MarkNotificationAsRead(ctx context.Context, params corev1.MarkNotificationAsReadParams) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.notificationsUC.MarkAsRead(ctx, params.ID, user.Id)
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to mark notification as read")
	}

	return nil
}

func (h *CoreServer) MarkAllNotificationsAsRead(ctx context.Context) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.notificationsUC.MarkAllAsRead(ctx, user.Id)
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to mark all notifications as read")
	}

	return nil
}
