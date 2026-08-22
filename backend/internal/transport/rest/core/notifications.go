package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
)

func (h *CoreServer) ListNotifications(ctx context.Context, request corev1.ListNotificationsRequestObject) (corev1.ListNotificationsResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	page := int32(1)
	if request.Params.Page != nil && *request.Params.Page > 0 {
		page = *request.Params.Page
	}

	pageSize := int32(20)
	if request.Params.PageSize != nil && *request.Params.PageSize > 0 {
		pageSize = *request.Params.PageSize
	}

	unreadOnly := false
	if request.Params.UnreadOnly != nil {
		unreadOnly = *request.Params.UnreadOnly
	}

	list, err := h.notificationsUC.ListNotifications(ctx, user.Id, &models.NotificationFilter{
		Page:       page,
		PageSize:   pageSize,
		UnreadOnly: unreadOnly,
	})
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list notifications")
	}

	return corev1.ListNotifications200JSONResponse(*NotificationsListResponseDTO(list)), nil
}

func (h *CoreServer) GetUnreadNotificationsCount(ctx context.Context, request corev1.GetUnreadNotificationsCountRequestObject) (corev1.GetUnreadNotificationsCountResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return corev1.GetUnreadNotificationsCount200JSONResponse{Count: 0}, nil
	}

	count, err := h.notificationsUC.GetUnreadCount(ctx, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get unread notifications count")
	}

	return corev1.GetUnreadNotificationsCount200JSONResponse{Count: count}, nil
}

func (h *CoreServer) MarkNotificationAsRead(ctx context.Context, request corev1.MarkNotificationAsReadRequestObject) (corev1.MarkNotificationAsReadResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.notificationsUC.MarkAsRead(ctx, request.Id, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to mark notification as read")
	}

	return corev1.MarkNotificationAsRead200Response{}, nil
}

func (h *CoreServer) MarkAllNotificationsAsRead(ctx context.Context, request corev1.MarkAllNotificationsAsReadRequestObject) (corev1.MarkAllNotificationsAsReadResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	err := h.notificationsUC.MarkAllAsRead(ctx, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to mark all notifications as read")
	}

	return corev1.MarkAllNotificationsAsRead200Response{}, nil
}
