//go:build integration

package integration

import (
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
)

func (s *IntegrationTestSuite) TestCreateContestRollsBackOwnerFailure() {
	owner := s.createUser("atomic_contest_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("atomic-contest", "Atomic contest", owner.Id)
	_, err := s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_contest_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected contest owner failure'; END $$;
CREATE TRIGGER reject_contest_owner BEFORE INSERT ON contest_members FOR EACH ROW EXECUTE FUNCTION reject_contest_owner()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_contest_owner ON contest_members; DROP FUNCTION reject_contest_owner()`)
		s.Require().NoError(err)
	}()
	uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
	_, err = uc.CreateContest(s.ctx, &contestsmodel.CreateContestInput{OrganizationID: org.ID, OwnerID: &owner.Id, Title: "Atomic contest", Login: "atomic", Visibility: contestsmodel.ContestVisibilityPublic})
	s.Require().Error(err)
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM contests WHERE organization_id=$1`, org.ID).Scan(&count))
	s.Zero(count)
}

func (s *IntegrationTestSuite) TestProblemBlockRollsBackSubmissionAndStandingsOnEventFailure() {
	sub, contest, problem, user := s.newJobSubmission()
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	_, err := repo.UpdateSubmission(s.ctx, sub, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.Accepted})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.UpsertContestProblemResult(s.ctx, &contestsmodel.UpsertContestProblemResultParams{ContestID: contest, UserID: user, ProblemID: problem, Solved: true}))
	before, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_block_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected block event failure'; END $$;
CREATE TRIGGER reject_block_event BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION reject_block_event()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_block_event ON outbox_events; DROP FUNCTION reject_block_event()`)
		s.Require().NoError(err)
	}()
	uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
	s.Require().Error(uc.BlockProblemForUser(s.ctx, contest, user, problem, nil, user))
	after, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(before.State, after.State)
	s.Equal(before.Generation, after.Generation)
	s.Equal(before.Revision, after.Revision)
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM contest_user_problem_blocks WHERE contest_id=$1 AND user_id=$2 AND problem_id=$3`, contest, user, problem).Scan(&count))
	s.Zero(count)
	standing, err := s.contestsRepo.GetContestProblemResult(s.ctx, contest, user, problem)
	s.Require().NoError(err)
	s.True(standing.Solved)
}
