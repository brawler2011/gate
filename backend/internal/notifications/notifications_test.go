package notifications

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNotificationsRepo struct {
	notifications map[uuid.UUID]notificationsmodel.Notification
}

func newMockNotificationsRepo() *mockNotificationsRepo {
	return &mockNotificationsRepo{
		notifications: make(map[uuid.UUID]notificationsmodel.Notification),
	}
}

func (m *mockNotificationsRepo) CreateNotification(ctx context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	n := notificationsmodel.Notification{
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

func (m *mockNotificationsRepo) GetNotificationByID(ctx context.Context, id, userID uuid.UUID) (*notificationsmodel.Notification, error) {
	if n, ok := m.notifications[id]; ok && n.UserID == userID {
		return &n, nil
	}
	return nil, nil
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

func TestNotificationsUseCase_EmailAndRead(t *testing.T) {
	ctx := context.Background()
	notifsRepo := newMockNotificationsRepo()
	usersRepo := newMockUsersRepoForTemp()
	emailSvc := &mockEmailService{}

	userID := uuid.New()
	userEmail := "user@example.com"
	_ = usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{
		Id:           userID,
		Username:     "testuser",
		Role:         accessmodel.UserRoleUser,
		PasswordHash: "pass",
		Email:        &userEmail,
	})

	uc := NewNotificationsUseCase(notifsRepo, usersRepo, emailSvc)

	// Create in-app notification
	link := "/org/settings"
	notif, err := notifsRepo.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
		UserID: userID,
		Type:   notificationsmodel.NotificationTypeOrgInvitation,
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

	uc.SendEmail(ctx, *notif)

	// Mark as read
	err = uc.MarkAsRead(accessmodel.WithUser(ctx, accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}), notif.ID)
	require.NoError(t, err)
	assert.True(t, notifsRepo.notifications[notif.ID].IsRead)
}
