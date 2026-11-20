package postgres

import (
	"context"
	"errors"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationsRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewNotificationsRepo(pool *pgxpool.Pool) *NotificationsRepo {
	return &NotificationsRepo{
		queries: sqlc.New(pool), pool: pool,
	}
}

func (r *NotificationsRepo) CreateNotification(ctx context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	dataBytes, err := notificationsmodel.MarshalNotificationData(input.Data)
	if err != nil {
		return nil, dbError("marshal notification data", err)
	}

	id := uuid.New()
	row, err := r.queries.CreateNotification(ctx, sqlc.CreateNotificationParams{
		ID:     id,
		UserID: input.UserID,
		Type:   string(input.Type),
		Title:  input.Title,
		Body:   input.Body,
		Link:   input.Link,
		Data:   dataBytes,
		IsRead: false,
	})
	if err != nil {
		return nil, dbError("create notification", err)
	}

	return &notificationsmodel.Notification{
		ID:        row.ID,
		UserID:    row.UserID,
		Type:      notificationsmodel.NotificationType(row.Type),
		Title:     row.Title,
		Body:      row.Body,
		Link:      row.Link,
		Data:      notificationsmodel.UnmarshalNotificationData(row.Data),
		IsRead:    row.IsRead,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *NotificationsRepo) ListNotifications(ctx context.Context, userID uuid.UUID, filter *notificationsmodel.NotificationFilter) (*notificationsmodel.NotificationsList, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, userID); err != nil {
		return nil, err
	}
	if filter == nil {
		filter = &notificationsmodel.NotificationFilter{}
	}
	normalized := *filter
	if normalized.Page == 0 {
		normalized.Page = 1
	}
	if normalized.PageSize == 0 {
		normalized.PageSize = 20
	}
	if normalized.Page < 1 || normalized.PageSize < 1 || normalized.PageSize > 100 {
		return nil, pkg.ErrInvalid
	}
	filter = &normalized
	if r.pool != nil {
		return readSnapshot(ctx, r.pool, func(q *sqlc.Queries) (*notificationsmodel.NotificationsList, error) {
			return (&NotificationsRepo{queries: q}).ListNotifications(ctx, userID, filter)
		})
	}
	offset := Offset(filter.Page, filter.PageSize)
	var unreadOnly *bool
	if filter.UnreadOnly {
		t := true
		unreadOnly = &t
	}

	rows, err := r.queries.ListNotificationsByUserID(ctx, sqlc.ListNotificationsByUserIDParams{
		UserID:     userID,
		UnreadOnly: unreadOnly,
		Limit:      filter.PageSize,
		Offset:     offset,
	})
	if err != nil {
		return nil, dbError("list notifications", err)
	}

	total, err := r.queries.CountNotificationsByUserID(ctx, sqlc.CountNotificationsByUserIDParams{
		UserID:     userID,
		UnreadOnly: unreadOnly,
	})
	if err != nil {
		return nil, dbError("count notifications", err)
	}

	notifs := make([]notificationsmodel.Notification, len(rows))
	for i, row := range rows {
		notifs[i] = notificationsmodel.Notification{
			ID:        row.ID,
			UserID:    row.UserID,
			Type:      notificationsmodel.NotificationType(row.Type),
			Title:     row.Title,
			Body:      row.Body,
			Link:      row.Link,
			Data:      notificationsmodel.UnmarshalNotificationData(row.Data),
			IsRead:    row.IsRead,
			CreatedAt: row.CreatedAt,
		}
	}

	return &notificationsmodel.NotificationsList{
		Notifications: notifs,
		Pagination:    paging.NewPagination(filter.Page, filter.PageSize, safeInt32(total)),
	}, nil
}

func (r *NotificationsRepo) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int32, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, userID); err != nil {
		return 0, err
	}
	count, err := r.queries.GetUnreadNotificationsCount(ctx, userID)
	if err != nil {
		return 0, dbError("get unread notifications count", err)
	}
	return safeInt32(count), nil
}

func (r *NotificationsRepo) GetNotificationByID(ctx context.Context, id, userID uuid.UUID) (*notificationsmodel.Notification, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, userID); err != nil {
		return nil, err
	}
	row, err := r.queries.GetNotificationByID(ctx, sqlc.GetNotificationByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, dbError("get notification by id", err)
	}

	return &notificationsmodel.Notification{
		ID:        row.ID,
		UserID:    row.UserID,
		Type:      notificationsmodel.NotificationType(row.Type),
		Title:     row.Title,
		Body:      row.Body,
		Link:      row.Link,
		Data:      notificationsmodel.UnmarshalNotificationData(row.Data),
		IsRead:    row.IsRead,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *NotificationsRepo) MarkAsRead(ctx context.Context, id, userID uuid.UUID) error {
	err := r.queries.MarkNotificationAsRead(ctx, sqlc.MarkNotificationAsReadParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return dbError("mark notification as read", err)
	}
	return nil
}

func (r *NotificationsRepo) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	err := r.queries.MarkAllNotificationsAsRead(ctx, userID)
	if err != nil {
		return dbError("mark all notifications as read", err)
	}
	return nil
}
