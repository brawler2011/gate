//go:build integration

package integration

import (
	"context"
	"errors"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestCreateTeamRollsBackMaintainerFailure() {
	owner := s.createUser("atomic_team_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("atomic-team", "Atomic team", owner.Id)
	_, err := s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_team_member() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected member failure'; END $$;
CREATE TRIGGER reject_team_member BEFORE INSERT ON team_members FOR EACH ROW EXECUTE FUNCTION reject_team_member()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_team_member ON team_members; DROP FUNCTION reject_team_member()`)
		s.Require().NoError(err)
	}()
	_, err = s.client.CreateTeam(withTestUser(s.ctx, owner.Id), &corev1.CreateTeamReq{
		OrganizationID: org.ID, Name: "Atomic team",
	})
	s.Require().Error(err)
	var count int
	err = s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM teams WHERE organization_id=$1`, org.ID).Scan(&count)
	s.Require().NoError(err)
	s.Zero(count, "a failed maintainer insert must roll back the team")
}

func (s *IntegrationTestSuite) TestCreateProblemRollsBackOwnerFailure() {
	owner := s.createUser("atomic_problem_owner", accessmodel.UserRoleAdmin)
	org := s.createOrganization("atomic-problem", "Atomic problem", owner.Id)
	_, err := s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_problem_member() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected owner failure'; END $$;
CREATE TRIGGER reject_problem_member BEFORE INSERT ON problem_members FOR EACH ROW EXECUTE FUNCTION reject_problem_member()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_problem_member ON problem_members; DROP FUNCTION reject_problem_member()`)
		s.Require().NoError(err)
	}()
	_, err = s.client.CreateProblem(withTestUser(s.ctx, owner.Id), corev1.CreateProblemParams{
		Title: "Atomic problem", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b",
	})
	s.Require().Error(err)
	var count int
	err = s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM problems WHERE organization_id=$1`, org.ID).Scan(&count)
	s.Require().NoError(err)
	s.Zero(count, "a failed owner insert must roll back the problem")
}

func (s *IntegrationTestSuite) TestUsersPaginationDefaultsAndBounds() {
	admin := s.createUser("pagination_admin", accessmodel.UserRoleAdmin)
	s.readJSON("/users", admin.Id, 200)
	for _, query := range []string{"page=0", "pageSize=0", "pageSize=101"} {
		s.readJSON("/users?"+query, admin.Id, 400)
	}
	s.readJSON("/users?pageSize=100", admin.Id, 200)
}

func (s *IntegrationTestSuite) TestTransactionRollbackLifecycle() {
	owner := s.createUser("rollback_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("rollback-lifecycle", "Rollback", owner.Id)
	provider := pgstore.NewProblemTransactions(s.dbPool)
	failure := errors.New("injected callback failure")
	for _, mode := range []string{"error", "panic", "canceled"} {
		s.Run(mode, func() {
			id := uuid.New()
			ctx, cancel := context.WithCancel(s.ctx)
			defer cancel()
			callback := func(tx problems.ProblemTxStore) error {
				err := tx.CreateProblem(ctx, &problemsmodel.CreateProblemParams{ID: id, OrganizationID: org.ID, Title: "Rollback", ShortName: mode, Visibility: problemsmodel.ProblemVisibilityPrivate})
				if err != nil {
					return err
				}
				if mode == "panic" {
					panic(failure)
				}
				if mode == "canceled" {
					cancel()
				}
				return failure
			}
			if mode == "panic" {
				s.PanicsWithValue(failure, func() { _ = provider.Within(ctx, callback) })
			} else {
				err := provider.Within(ctx, callback)
				s.Same(failure, err, "preserve the original callback error")
			}
			checkCtx, stop := context.WithTimeout(s.ctx, time.Second)
			defer stop()
			var count int
			err := s.dbPool.QueryRow(checkCtx, `SELECT count(*) FROM problems WHERE id=$1`, id).Scan(&count)
			s.Require().NoError(err)
			s.Zero(count)
			s.Zero(s.dbPool.Stat().AcquiredConns(), "rollback must release the connection")
		})
	}
}
