//go:build integration

package integration

import (
	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
)

func (s *IntegrationTestSuite) TestPrivateReadersRejectSpoofedOwnersOutsideHTTP() {
	owner := s.createUser("private_reader_owner", accessmodel.UserRoleUser)
	other := s.createUser("private_reader_other", accessmodel.UserRoleUser)
	ownerCtx := accessmodel.WithUser(s.ctx, owner)
	otherCtx := accessmodel.WithUser(s.ctx, other)
	notifications := pgstore.NewNotificationsRepo(s.dbPool)
	notification, err := notifications.CreateNotification(s.ctx, &notificationsmodel.CreateNotificationInput{UserID: owner.Id, Type: notificationsmodel.NotificationTypeSystem, Title: "Private", Body: "Secret"})
	s.Require().NoError(err)
	for _, ctx := range []struct {
		name     string
		actor    accessmodel.User
		expected error
	}{
		{"guest", accessmodel.Guest, pkg.ErrUnauthenticated},
		{"other", other, pkg.NoPermission},
	} {
		s.Run(ctx.name, func() {
			actorCtx := accessmodel.WithUser(s.ctx, ctx.actor)
			_, err := notifications.ListNotifications(actorCtx, owner.Id, nil)
			s.ErrorIs(err, ctx.expected)
			_, err = notifications.GetUnreadCount(actorCtx, owner.Id)
			s.ErrorIs(err, ctx.expected)
			_, err = notifications.GetNotificationByID(actorCtx, notification.ID, owner.Id)
			s.ErrorIs(err, ctx.expected)
			_, err = s.usersRepo.ListClaimedAccountsByUserId(actorCtx, owner.Id)
			s.ErrorIs(err, ctx.expected)
		})
	}
	list, err := notifications.ListNotifications(ownerCtx, owner.Id, nil)
	s.Require().NoError(err)
	s.Require().Len(list.Notifications, 1)
	s.Equal(notification.ID, list.Notifications[0].ID)
	s.EqualValues(1, list.Pagination.TotalPages)
	count, err := notifications.GetUnreadCount(ownerCtx, owner.Id)
	s.Require().NoError(err)
	s.EqualValues(1, count)
	_, err = notifications.ListNotifications(ownerCtx, owner.Id, &notificationsmodel.NotificationFilter{Page: -1})
	s.ErrorIs(err, pkg.ErrBadInput)
	_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET claimed_by_user_id=$1,claimed_at=now(),expires_at=now()+interval '1 day' WHERE id=$2", owner.Id, other.Id)
	s.Require().NoError(err)
	accounts, err := s.usersRepo.ListClaimedAccountsByUserId(ownerCtx, owner.Id)
	s.Require().NoError(err)
	s.Require().Len(accounts, 1)
	s.Equal(other.Id, accounts[0].ID)
	accounts, err = s.usersRepo.ListClaimedAccountsByUserId(otherCtx, other.Id)
	s.Require().NoError(err)
	s.Empty(accounts)
}

func (s *IntegrationTestSuite) TestDraftListChecksCurrentAccessOutsideHTTP() {
	_, contest, _, owner := s.newJobSubmission()
	other := s.createUser("draft_reader_other", accessmodel.UserRoleUser)
	ownerCtx := accessmodel.WithUser(s.ctx, accessmodel.User{Id: owner, Role: accessmodel.UserRoleUser})
	repo := pgstore.NewDraftsRepo(s.dbPool)
	id, err := repo.CreateDraft(s.ctx, &contestsmodel.ContestDraftCreation{ContestID: contest, UserID: owner, Code: "private"})
	s.Require().NoError(err)
	reader := contests.NewDraftsUseCase(pgstore.NewDraftTransactions(s.dbPool), pgstore.NewDraftsRepo(s.dbPool), access.NewPermissionsUseCase(s.contestsRepo, s.usersRepo, s.problemsRepo, s.teamsRepo, s.organizationsRepo))
	filter := contestsmodel.ContestDraftsFilter{ContestID: contest, UserID: owner}
	_, err = reader.ListDrafts(s.ctx, filter)
	s.ErrorIs(err, pkg.ErrUnauthenticated)
	_, err = reader.ListDrafts(accessmodel.WithUser(s.ctx, other), filter)
	s.ErrorIs(err, pkg.NoPermission)
	list, err := reader.ListDrafts(ownerCtx, filter)
	s.Require().NoError(err)
	s.Require().Len(list.Drafts, 1)
	s.Equal(id, list.Drafts[0].ID)
	// Remove current access while preserving the author's draft.
	_, err = s.dbPool.Exec(s.ctx, "UPDATE contests SET visibility='private' WHERE id=$1", contest)
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, "DELETE FROM contest_members WHERE contest_id=$1", contest)
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, "DELETE FROM organization_members WHERE organization_id=(SELECT organization_id FROM contests WHERE id=$1)", contest)
	s.Require().NoError(err)
	_, err = reader.ListDrafts(ownerCtx, filter)
	s.ErrorIs(err, pkg.NoPermission)
}
