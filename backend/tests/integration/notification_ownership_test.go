//go:build integration

package integration

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
)

func (s *IntegrationTestSuite) TestNotificationReadCommandsOnlyUpdateOwnNotifications() {
	caller := s.createUser("notification_caller", accessmodel.UserRoleAdmin)
	other := s.createUser("notification_other", accessmodel.UserRoleUser)
	repo := pgstore.NewNotificationsRepo(s.dbPool)
	own, err := repo.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: caller.Id, Type: notificationsmodel.NotificationTypeOrgInvitation, Title: "Own"})
	s.Require().NoError(err)
	foreign, err := repo.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: other.Id, Type: notificationsmodel.NotificationTypeOrgInvitation, Title: "Other"})
	s.Require().NoError(err)
	ctx := withTestUser(s.ctx, caller.Id)
	s.Require().NoError(s.client.MarkNotificationAsRead(ctx, corev1.MarkNotificationAsReadParams{ID: foreign.ID}))
	foreign, err = repo.GetNotificationByID(accessmodel.WithUser(s.ctx, other), foreign.ID, other.Id)
	s.Require().NoError(err)
	s.False(foreign.IsRead)
	for range 2 {
		s.Require().NoError(s.client.MarkNotificationAsRead(ctx, corev1.MarkNotificationAsReadParams{ID: own.ID}))
		s.Require().NoError(s.client.MarkAllNotificationsAsRead(ctx))
	}
	own, err = repo.GetNotificationByID(accessmodel.WithUser(s.ctx, caller), own.ID, caller.Id)
	s.Require().NoError(err)
	s.True(own.IsRead)
	foreign, err = repo.GetNotificationByID(accessmodel.WithUser(s.ctx, other), foreign.ID, other.Id)
	s.Require().NoError(err)
	s.False(foreign.IsRead)
}
