package notifications

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type eligibilityUsers struct {
	user accessmodel.User
	err  error
}

func (u eligibilityUsers) GetUserById(context.Context, uuid.UUID) (accessmodel.User, error) {
	return u.user, u.err
}

func TestNotificationCommandsUseCurrentActorEligibility(t *testing.T) {
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleAdmin}
	expired := time.Now().Add(-time.Hour)
	for _, test := range []struct {
		name     string
		current  accessmodel.User
		readErr  error
		expected error
	}{
		{name: "expired", current: accessmodel.User{Id: actor.Id, Role: actor.Role, ExpiresAt: &expired}, expected: pkg.ErrUnauthenticated},
		{name: "demoted", current: accessmodel.User{Id: actor.Id, Role: accessmodel.UserRoleUser}, expected: pkg.NoPermission},
		{name: "missing", readErr: pkg.ErrNotFound, expected: pkg.ErrNotFound},
		{name: "current_admin", current: actor},
	} {
		for _, all := range []bool{false, true} {
			t.Run(test.name+map[bool]string{false: "/one", true: "/all"}[all], func(t *testing.T) {
				repo := newMockNotificationsRepo()
				repo.UsersRepo = eligibilityUsers{user: test.current, err: test.readErr}
				target := uuid.New()
				notification, err := repo.CreateNotification(context.Background(), &notificationsmodel.CreateNotificationInput{UserID: target})
				require.NoError(t, err)
				uc := NewNotificationsUseCase(repo, nil, nil)
				ctx := accessmodel.WithUser(context.Background(), actor)
				if all {
					err = uc.MarkAllAsRead(ctx, target)
				} else {
					err = uc.MarkAsRead(ctx, notification.ID, target)
				}
				if test.expected == nil {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, test.expected)
				}
				require.Equal(t, test.expected == nil, repo.notifications[notification.ID].IsRead)
			})
		}
	}
}
