//go:build integration

package integration

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	"github.com/brawler2011/gate/backend/internal/organizations"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
)

type notificationEmailFunc func(context.Context, notificationsmodel.Notification)

func (f notificationEmailFunc) SendEmail(ctx context.Context, n notificationsmodel.Notification) {
	f(ctx, n)
}

func (s *IntegrationTestSuite) TestOrganizationNotificationFailureRollsBackCommand() {
	owner := s.createUser("notification_owner", accessmodel.UserRoleUser)
	applicant := s.createUser("notification_applicant", accessmodel.UserRoleUser)
	org := s.createOrganization("notification-atomic", "Notification", owner.Id)
	permissions := access.NewPermissionsUseCase(s.contestsRepo, s.usersRepo, s.problemsRepo, pgstore.NewTeamsRepo(s.dbPool), s.organizationsRepo)
	emails := 0
	mailer := notificationEmailFunc(func(ctx context.Context, n notificationsmodel.Notification) {
		emails++
		var count int
		err := s.dbPool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE id=$1`, n.ID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(1, count, "email must run after the notification commit")
	})
	uc := organizations.NewOrganizationsUseCase(s.organizationsRepo, s.usersRepo, permissions, pgstore.NewOrganizationTransactions(s.dbPool), mailer)
	ctx := accessmodel.WithUser(s.ctx, owner)
	_, err := s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_notification() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected notification failure'; END $$;
 CREATE TRIGGER reject_notification BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION reject_notification()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER IF EXISTS reject_notification ON notifications; DROP FUNCTION reject_notification()`)
		s.Require().NoError(err)
	}()
	_, err = uc.InviteMember(ctx, org.Login, applicant.Id, organizationsmodel.OrgRoleMember, owner.Id)
	s.Require().Error(err)
	pending, err := s.organizationsRepo.GetPendingInvitation(s.ctx, org.ID, applicant.Id)
	s.Require().NoError(err)
	s.Nil(pending, "notification failure must roll back the invitation")
	s.Zero(emails)
	_, _, err = uc.CreateJoinRequest(accessmodel.WithUser(s.ctx, applicant), org.Login, nil)
	s.Require().Error(err)
	request, err := s.organizationsRepo.GetPendingJoinRequest(s.ctx, org.ID, applicant.Id)
	s.Require().NoError(err)
	s.Nil(request, "notification failure must roll back the join request")
	request, err = s.organizationsRepo.CreateJoinRequest(s.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{OrganizationID: org.ID, UserID: applicant.Id})
	s.Require().NoError(err)
	s.Require().Error(uc.ApproveJoinRequest(ctx, org.Login, request.ID, owner.Id, organizationsmodel.OrgRoleMember))
	_, err = s.organizationsRepo.GetMember(s.ctx, org.ID, applicant.Id)
	s.Require().ErrorIs(err, pkg.ErrNotFound)
	stored, err := s.organizationsRepo.GetJoinRequestByID(s.ctx, request.ID)
	s.Require().NoError(err)
	s.Equal(organizationsmodel.RequestStatusPending, stored.Status)
	s.Zero(emails)
	_, err = s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_notification ON notifications`)
	s.Require().NoError(err)
	s.Require().NoError(uc.ApproveJoinRequest(ctx, org.Login, request.ID, owner.Id, organizationsmodel.OrgRoleMember))
	s.Equal(1, emails)
}

func (s *IntegrationTestSuite) TestContestNotificationFailureRollsBackApproval() {
	_, contestID, _, ownerID := s.newJobSubmission()
	applicant := s.createUser("contest_notify_applicant", accessmodel.UserRoleUser)
	contest, err := s.contestsRepo.GetContest(s.ctx, contestID)
	s.Require().NoError(err)
	org, err := s.organizationsRepo.GetOrganizationByID(s.ctx, contest.OrganizationID)
	s.Require().NoError(err)
	req, err := s.contestsRepo.CreateContestJoinRequest(s.ctx, &contestsmodel.CreateContestJoinRequestInput{ContestID: contestID, UserID: applicant.Id})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_contest_notification() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected notification failure'; END $$;
 CREATE TRIGGER reject_contest_notification BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION reject_contest_notification()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_contest_notification ON notifications; DROP FUNCTION reject_contest_notification()`)
		s.Require().NoError(err)
	}()
	uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
	ctx := accessmodel.WithUser(s.ctx, accessmodel.User{Id: ownerID, Role: accessmodel.UserRoleUser})
	s.Require().Error(uc.ApproveJoinRequest(ctx, org.Login, contest.Login, req.ID, ownerID))
	stored, err := s.contestsRepo.GetContestJoinRequestByID(s.ctx, req.ID)
	s.Require().NoError(err)
	s.Equal(organizationsmodel.RequestStatusPending, stored.Status)
	_, err = s.contestsRepo.GetContestMember(s.ctx, &contestsmodel.ContestPermissionGet{ContestId: contestID, UserId: applicant.Id})
	s.ErrorIs(err, pkg.ErrNotFound)
}
