//go:build integration

package integration

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
)

func (s *IntegrationTestSuite) TestCreateTeamRollsBackMaintainerFailure() {
	owner := s.createUser("atomic_team_owner", models.UserRoleUser)
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
	owner := s.createUser("atomic_problem_owner", models.UserRoleAdmin)
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
	admin := s.createUser("pagination_admin", models.UserRoleAdmin)
	s.readJSON("/users", admin.Id, 200)
	for _, query := range []string{"page=0", "page_size=0", "page_size=101"} {
		s.readJSON("/users?"+query, admin.Id, 400)
	}
	s.readJSON("/users?page_size=100", admin.Id, 200)
}
