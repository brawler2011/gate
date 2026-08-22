package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
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
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *NotificationsRepo) WithTx(tx pgx.Tx) interfaces.NotificationsRepo {
	return &NotificationsRepo{
		pool:    r.pool,
		queries: r.queries.WithTx(tx),
	}
}

func (r *NotificationsRepo) CreateNotification(ctx context.Context, input *models.CreateNotificationInput) (*models.Notification, error) {
	dataBytes, err := models.MarshalNotificationData(input.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal notification data: %w", err)
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
		return nil, fmt.Errorf("create notification: %w", err)
	}

	return &models.Notification{
		ID:        row.ID,
		UserID:    row.UserID,
		Type:      models.NotificationType(row.Type),
		Title:     row.Title,
		Body:      row.Body,
		Link:      row.Link,
		Data:      models.UnmarshalNotificationData(row.Data),
		IsRead:    row.IsRead,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *NotificationsRepo) ListNotifications(ctx context.Context, userID uuid.UUID, filter *models.NotificationFilter) (*models.NotificationsList, error) {
	offset := (filter.Page - 1) * filter.PageSize
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
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	total, err := r.queries.CountNotificationsByUserID(ctx, sqlc.CountNotificationsByUserIDParams{
		UserID:     userID,
		UnreadOnly: unreadOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("count notifications: %w", err)
	}

	notifs := make([]models.Notification, len(rows))
	for i, row := range rows {
		notifs[i] = models.Notification{
			ID:        row.ID,
			UserID:    row.UserID,
			Type:      models.NotificationType(row.Type),
			Title:     row.Title,
			Body:      row.Body,
			Link:      row.Link,
			Data:      models.UnmarshalNotificationData(row.Data),
			IsRead:    row.IsRead,
			CreatedAt: row.CreatedAt,
		}
	}

	return &models.NotificationsList{
		Notifications: notifs,
		Pagination:    models.NewPagination(filter.Page, filter.PageSize, safeInt32(total)),
	}, nil
}

func (r *NotificationsRepo) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int32, error) {
	count, err := r.queries.GetUnreadNotificationsCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get unread notifications count: %w", err)
	}
	return safeInt32(count), nil
}

func (r *NotificationsRepo) GetNotificationByID(ctx context.Context, id, userID uuid.UUID) (*models.Notification, error) {
	row, err := r.queries.GetNotificationByID(ctx, sqlc.GetNotificationByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, fmt.Errorf("get notification by id: %w", err)
	}

	return &models.Notification{
		ID:        row.ID,
		UserID:    row.UserID,
		Type:      models.NotificationType(row.Type),
		Title:     row.Title,
		Body:      row.Body,
		Link:      row.Link,
		Data:      models.UnmarshalNotificationData(row.Data),
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
		return fmt.Errorf("mark notification as read: %w", err)
	}
	return nil
}

func (r *NotificationsRepo) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	err := r.queries.MarkAllNotificationsAsRead(ctx, userID)
	if err != nil {
		return fmt.Errorf("mark all notifications as read: %w", err)
	}
	return nil
}
