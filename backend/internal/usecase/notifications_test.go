package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNotificationsRepo struct {
	interfaces.NotificationsRepo
	notifications map[uuid.UUID]models.Notification
}

func newMockNotificationsRepo() *mockNotificationsRepo {
	return &mockNotificationsRepo{
		notifications: make(map[uuid.UUID]models.Notification),
	}
}

func (m *mockNotificationsRepo) WithTx(tx pgx.Tx) interfaces.NotificationsRepo {
	return m
}

func (m *mockNotificationsRepo) CreateNotification(ctx context.Context, input *models.CreateNotificationInput) (*models.Notification, error) {
	n := models.Notification{
		ID:        uuid.New(),
		UserID:    input.UserID,
		Type:      input.Type,
		Title:     input.Title,
		Body:      input.Body,
		Link:      input.Link,
		Data:      input.Data,
		IsRead:    false,
		CreatedAt: time.Now(),
	}
	m.notifications[n.ID] = n
	return &n, nil
}

func (m *mockNotificationsRepo) GetNotificationByID(ctx context.Context, id, userID uuid.UUID) (*models.Notification, error) {
	if n, ok := m.notifications[id]; ok && n.UserID == userID {
		return &n, nil
	}
	return nil, nil
}

func (m *mockNotificationsRepo) ListNotifications(ctx context.Context, userID uuid.UUID, filter *models.NotificationFilter) (*models.NotificationsList, error) {
	var list []models.Notification
	for _, n := range m.notifications {
		if n.UserID == userID {
			if filter.UnreadOnly && n.IsRead {
				continue
			}
			list = append(list, n)
		}
	}
	return &models.NotificationsList{
		Notifications: list,
		Pagination:    models.NewPagination(filter.Page, filter.PageSize, int32(len(list))), //nolint:gosec // test slice length
	}, nil
}

func (m *mockNotificationsRepo) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int32, error) {
	var count int32
	for _, n := range m.notifications {
		if n.UserID == userID && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (m *mockNotificationsRepo) MarkAsRead(ctx context.Context, id, userID uuid.UUID) error {
	if n, ok := m.notifications[id]; ok && n.UserID == userID {
		n.IsRead = true
		m.notifications[id] = n
	}
	return nil
}

func (m *mockNotificationsRepo) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	for id, n := range m.notifications {
		if n.UserID == userID {
			n.IsRead = true
			m.notifications[id] = n
		}
	}
	return nil
}

func TestNotificationsUseCase_NotifyAndRead(t *testing.T) {
	ctx := context.Background()
	notifsRepo := newMockNotificationsRepo()
	usersRepo := newMockUsersRepoForTemp()
	emailSvc := &mockEmailService{}

	userID := uuid.New()
	userEmail := "user@example.com"
	_ = usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:           userID,
		Username:     "testuser",
		Role:         models.UserRoleUser,
		PasswordHash: "pass",
		Email:        &userEmail,
	})

	uc := NewNotificationsUseCase(notifsRepo, usersRepo, emailSvc)

	// Create in-app notification
	link := "/org/settings"
	notif, err := uc.Notify(ctx, &models.CreateNotificationInput{
		UserID: userID,
		Type:   models.NotificationTypeOrgInvitation,
		Title:  "Invite",
		Body:   "You have been invited",
		Link:   &link,
		Data: map[string]interface{}{
			"organization_name":  "TestOrg",
			"organization_login": "test-org",
			"role":               "member",
			"inviter_username":   "admin",
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, notif)
	assert.False(t, notif.IsRead)

	// Check unread count
	count, err := uc.GetUnreadCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), count)

	// List notifications
	list, err := uc.ListNotifications(ctx, userID, &models.NotificationFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, list.Notifications, 1)

	// Mark as read
	err = uc.MarkAsRead(ctx, notif.ID, userID)
	require.NoError(t, err)

	// Check unread count is 0
	count, err = uc.GetUnreadCount(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), count)
}
