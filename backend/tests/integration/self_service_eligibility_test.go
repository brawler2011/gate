//go:build integration

package integration

import (
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/organizations"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestOrganizationSelfServiceRechecksCurrentActor() {
	for _, reason := range []string{"expired", "demoted_admin"} {
		for _, operation := range []string{"accept", "decline", "create_open", "create_request", "cancel"} {
			s.Run(reason+"/"+operation, func() {
				suffix := uuid.NewString()[:8]
				owner := s.createUser("self_owner_"+suffix, accessmodel.UserRoleUser)
				target := s.createUser("self_target_"+suffix, accessmodel.UserRoleUser)
				actor := target
				if reason == "demoted_admin" {
					actor = s.createUser("self_admin_"+suffix, accessmodel.UserRoleAdmin)
				}
				ctx := accessmodel.WithUser(s.ctx, actor)
				org := s.createOrganization("self-"+suffix, "Self service", owner.Id)
				policy := organizationsmodel.OrgJoinPolicyByRequest
				if operation == "create_open" {
					policy = organizationsmodel.OrgJoinPolicyOpen
				}
				s.Require().NoError(s.organizationsRepo.UpdateOrganization(s.ctx, org.ID, &organizationsmodel.UpdateOrganizationInput{JoinPolicy: &policy}))
				var invitationID uuid.UUID
				if operation == "accept" || operation == "decline" {
					invitation, err := s.organizationsRepo.CreateInvitation(s.ctx, &organizationsmodel.CreateOrganizationInvitationInput{OrganizationID: org.ID, UserID: target.Id, InviterID: owner.Id, Role: organizationsmodel.OrgRoleMember})
					s.Require().NoError(err)
					invitationID = invitation.ID
				}
				if operation == "cancel" {
					_, err := s.organizationsRepo.CreateJoinRequest(s.ctx, &organizationsmodel.CreateOrganizationJoinRequestInput{OrganizationID: org.ID, UserID: target.Id})
					s.Require().NoError(err)
				}
				uc := organizations.NewOrganizationsUseCase(s.organizationsRepo, s.usersRepo, nil, pgstore.NewOrganizationTransactions(s.dbPool), nil)
				command := func() error {
					switch operation {
					case "accept":
						return uc.AcceptInvitation(ctx, invitationID, target.Id)
					case "decline":
						return uc.DeclineInvitation(ctx, invitationID, target.Id)
					case "cancel":
						return uc.CancelJoinRequest(ctx, org.Login, target.Id)
					default:
						_, _, err := uc.CreateJoinRequest(ctx, org.Login, target.Id, nil)
						return err
					}
				}
				want := pkg.ErrUnauthenticated
				if reason == "expired" {
					_, err := s.dbPool.Exec(s.ctx, `UPDATE users SET expires_at=now()-interval '1 hour' WHERE id=$1`, actor.Id)
					s.Require().NoError(err)
				} else {
					_, err := s.dbPool.Exec(s.ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.Id)
					s.Require().NoError(err)
					want = pkg.NoPermission
				}
				s.ErrorIs(command(), want)
				_, err := s.organizationsRepo.GetMember(s.ctx, org.ID, target.Id)
				s.ErrorIs(err, pkg.ErrNotFound)
				if invitationID != uuid.Nil {
					invitation, err := s.organizationsRepo.GetInvitationByID(s.ctx, invitationID)
					s.Require().NoError(err)
					s.Equal(organizationsmodel.RequestStatusPending, invitation.Status)
				}
				pending, err := s.organizationsRepo.GetPendingJoinRequest(s.ctx, org.ID, target.Id)
				s.Require().NoError(err)
				if operation == "cancel" {
					s.Require().NotNil(pending)
				} else {
					s.Nil(pending)
				}
				_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET role=$2, expires_at=NULL WHERE id=$1`, actor.Id, actor.Role)
				s.Require().NoError(err)
				s.Require().NoError(command(), "an eligible actor retains the existing self/admin policy")
			})
		}
	}
}

func (s *IntegrationTestSuite) TestContestSelfServiceRejectsNewlyExpiredActor() {
	for _, operation := range []string{"open", "by_request", "cancel"} {
		s.Run(operation, func() {
			_, contestID, _, _ := s.newJobSubmission()
			contest, err := s.contestsRepo.GetContest(s.ctx, contestID)
			s.Require().NoError(err)
			target := s.createUser("contest_self_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
			s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, contest.OrganizationID, target.Id, organizationsmodel.OrgRoleMember))
			mode := contestsmodel.ParticipationModeByRequest
			if operation == "open" {
				mode = contestsmodel.ParticipationModeOpen
			}
			settings := map[string]interface{}{"participation_mode": mode}
			s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID, Settings: &settings}))
			if operation == "cancel" {
				_, err := s.contestsRepo.CreateContestJoinRequest(s.ctx, &contestsmodel.CreateContestJoinRequestInput{ContestID: contestID, UserID: target.Id})
				s.Require().NoError(err)
			}
			ctx := accessmodel.WithUser(s.ctx, target)
			uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
			command := func() error {
				if operation == "cancel" {
					return uc.CancelJoinRequest(ctx, contest.OrganizationLogin, contest.Login, target.Id)
				}
				_, _, err := uc.CreateJoinRequest(ctx, contest.OrganizationLogin, contest.Login, target.Id, nil)
				return err
			}
			_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET expires_at=now()-interval '1 hour' WHERE id=$1`, target.Id)
			s.Require().NoError(err)
			s.ErrorIs(command(), pkg.ErrUnauthenticated)
			_, err = s.contestsRepo.GetContestMember(s.ctx, &contestsmodel.ContestPermissionGet{ContestId: contestID, UserId: target.Id})
			s.ErrorIs(err, pkg.ErrNotFound)
			pending, err := s.contestsRepo.GetPendingContestJoinRequest(s.ctx, contestID, target.Id)
			s.Require().NoError(err)
			if operation == "cancel" {
				s.Require().NotNil(pending)
			} else {
				s.Nil(pending)
			}
			_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET expires_at=NULL WHERE id=$1`, target.Id)
			s.Require().NoError(err)
			s.Require().NoError(command())
		})
	}
}

func (s *IntegrationTestSuite) TestDraftCommandsRejectDemotedAdminPrivileges() {
	_, contestID, _, ownerID := s.newJobSubmission()
	admin := s.createUser("draft_old_admin_"+uuid.NewString()[:8], accessmodel.UserRoleAdmin)
	oldCtx := accessmodel.WithUser(s.ctx, admin)
	end := time.Now().Add(-time.Hour)
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID, EndTime: &end, EndTimeSet: true}))
	repo := pgstore.NewDraftsRepo(s.dbPool)
	draftID, err := repo.CreateDraft(s.ctx, &contestsmodel.ContestDraftCreation{ContestID: contestID, UserID: ownerID, Code: "owner code"})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET role='user' WHERE id=$1`, admin.Id)
	s.Require().NoError(err)
	uc := contests.NewDraftsUseCase(pgstore.NewDraftTransactions(s.dbPool))
	s.ErrorIs(uc.DeleteDraft(oldCtx, contestID, draftID), pkg.NoPermission)
	_, err = uc.CreateDraft(oldCtx, &contestsmodel.ContestDraftCreation{ContestID: contestID, UserID: admin.Id, Code: "late code"})
	s.ErrorIs(err, pkg.ErrBadInput)
	_, err = repo.GetDraft(s.ctx, draftID)
	s.Require().NoError(err)
	count, err := repo.GetDraftsCount(s.ctx, contestID, admin.Id)
	s.Require().NoError(err)
	s.Zero(count)
	_, err = s.dbPool.Exec(s.ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin.Id)
	s.Require().NoError(err)
	s.Require().NoError(uc.DeleteDraft(oldCtx, contestID, draftID))
	_, err = uc.CreateDraft(oldCtx, &contestsmodel.ContestDraftCreation{ContestID: contestID, UserID: admin.Id, Code: "admin code"})
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TestProblemBlockCannotSpoofOperator() {
	_, contestID, problemID, ownerID := s.newJobSubmission()
	owner, err := s.usersRepo.GetUserById(s.ctx, ownerID)
	s.Require().NoError(err)
	other := s.createUser("spoof_operator_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	ctx := accessmodel.WithUser(s.ctx, owner)
	uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
	s.ErrorIs(uc.BlockProblemForUser(ctx, contestID, ownerID, problemID, nil, other.Id), pkg.NoPermission)
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM contest_user_problem_blocks WHERE contest_id=$1`, contestID).Scan(&count))
	s.Zero(count)
	s.Require().NoError(uc.BlockProblemForUser(ctx, contestID, ownerID, problemID, nil, ownerID))
	block, err := s.contestsRepo.GetContestUserProblemBlock(s.ctx, contestID, ownerID, problemID)
	s.Require().NoError(err)
	s.Require().NotNil(block)
	s.Equal(&ownerID, block.CreatedBy)
}
