//go:build integration

package integration

import (
	"context"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/notifications"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestNotificationCommandsRecheckCurrentActor() {
	repo := pgstore.NewNotificationsRepo(s.dbPool)
	uc := notifications.NewNotificationsUseCase(pgstore.NewNotificationTransactions(s.dbPool), s.usersRepo, nil)
	for _, command := range []struct {
		name string
		run  func(context.Context, uuid.UUID, uuid.UUID) error
	}{
		{"one", uc.MarkAsRead},
		{"all", func(ctx context.Context, _ uuid.UUID, target uuid.UUID) error { return uc.MarkAllAsRead(ctx, target) }},
	} {
		for _, scenario := range []string{"expired_self", "demoted_admin", "eligible_self", "eligible_admin_expired_target"} {
			s.Run(command.name+"/"+scenario, func() {
				suffix := uuid.NewString()[:8]
				target := s.createUser("notif_target_"+suffix, accessmodel.UserRoleUser)
				actor := target
				if scenario == "demoted_admin" || scenario == "eligible_admin_expired_target" {
					actor = s.createUser("notif_admin_"+suffix, accessmodel.UserRoleAdmin)
				}
				oldCtx := accessmodel.WithUser(s.ctx, actor)
				var expected error
				switch scenario {
				case "expired_self":
					_, err := s.dbPool.Exec(s.ctx, `UPDATE users SET expires_at=now()-interval '1 hour' WHERE id=$1`, actor.Id)
					s.Require().NoError(err)
					expected = pkg.ErrUnauthenticated
				case "demoted_admin":
					_, err := s.dbPool.Exec(s.ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.Id)
					s.Require().NoError(err)
					expected = pkg.NoPermission
				case "eligible_admin_expired_target":
					_, err := s.dbPool.Exec(s.ctx, `UPDATE users SET expires_at=now()-interval '1 hour' WHERE id=$1`, target.Id)
					s.Require().NoError(err)
				}
				first, err := repo.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: target.Id, Type: notificationsmodel.NotificationTypeSystem, Title: "First", Body: "Unread"})
				s.Require().NoError(err)
				second, err := repo.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: target.Id, Type: notificationsmodel.NotificationTypeSystem, Title: "Second", Body: "Unread"})
				s.Require().NoError(err)
				other := s.createUser("notif_other_"+suffix, accessmodel.UserRoleUser)
				unrelated, err := repo.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: other.Id, Type: notificationsmodel.NotificationTypeSystem, Title: "Unrelated", Body: "Unread"})
				s.Require().NoError(err)
				err = command.run(oldCtx, first.ID, target.Id)
				if expected == nil {
					s.Require().NoError(err)
				} else {
					s.ErrorIs(err, expected)
				}
				for _, check := range []struct {
					id   uuid.UUID
					read bool
				}{
					{first.ID, expected == nil},
					{second.ID, expected == nil && command.name == "all"},
					{unrelated.ID, false},
				} {
					var read bool
					s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT is_read FROM notifications WHERE id=$1`, check.id).Scan(&read))
					s.Equal(check.read, read, "notification %s", check.id)
				}
			})
		}
	}
}
