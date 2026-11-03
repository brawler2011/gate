//go:build integration

package integration

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"sync"
	"time"

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
	_, err = uc.CreateContest(accessmodel.WithUser(s.ctx, owner), &contestsmodel.CreateContestInput{OrganizationID: org.ID, OwnerID: &owner.Id, Title: "Atomic contest", Login: "atomic", Visibility: contestsmodel.ContestVisibilityPublic})
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
	s.Require().Error(uc.BlockProblemForUser(accessmodel.WithUser(s.ctx, accessmodel.User{Id: user, Role: accessmodel.UserRoleUser}), contest, user, problem, nil, user))
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

func (s *IntegrationTestSuite) TestContestPatchRetainsConcurrentFieldsAndNullPresence() {
	_, contest, _, _ := s.newJobSubmission()
	initial := map[string]interface{}{"freeze_status": "frozen", "enable_drafts": true}
	start := time.Now().UTC().Truncate(time.Second)
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contest, Settings: &initial, StartTime: &start, StartTimeSet: true}))
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, patch := range []map[string]interface{}{{"enable_upsolving": true}, {"monitor_scope": "public"}} {
		wg.Go(func() {
			results <- s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contest, Settings: &patch})
		})
	}
	wg.Wait()
	close(results)
	for err := range results {
		s.Require().NoError(err)
	}
	got, err := s.contestsRepo.GetContest(s.ctx, contest)
	s.Require().NoError(err)
	s.Equal("frozen", got.Settings["freeze_status"])
	s.Equal(true, got.Settings["enable_drafts"])
	s.Equal(true, got.Settings["enable_upsolving"])
	s.Equal("public", got.Settings["monitor_scope"])
	s.Require().NotNil(got.StartTime)
	s.True(start.Equal(*got.StartTime), "absent datetime leaves the stored value")
	empty := ""
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contest, Description: &empty, StartTimeSet: true}))
	got, err = s.contestsRepo.GetContest(s.ctx, contest)
	s.Require().NoError(err)
	s.Nil(got.StartTime, "explicit null clears the datetime")
	s.Equal("", got.Description)
	s.Equal(true, got.Settings["enable_upsolving"], "absent settings keep existing fields")
}

func (s *IntegrationTestSuite) TestUnblockProblemQueuesOneGeneration() {
	sub, contestID, problemID, userID := s.newJobSubmission()
	actor, err := s.usersRepo.GetUserById(s.ctx, userID)
	s.Require().NoError(err)
	ctx := accessmodel.WithUser(s.ctx, actor)
	uc := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
	s.Require().NoError(uc.BlockProblemForUser(ctx, contestID, userID, problemID, nil, userID))
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	before, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	contest, err := s.contestsRepo.GetContest(s.ctx, contestID)
	s.Require().NoError(err)
	s.Require().NoError(s.client.UnblockProblemForUser(withTestUser(s.ctx, userID), corev1.UnblockProblemForUserParams{
		OrgLogin: contest.OrganizationLogin, ContestLogin: contest.Login, UserID: userID, ProblemID: problemID, RejudgeSubmissions: corev1.NewOptBool(true),
	}))
	after, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(before.Generation+1, after.Generation)
	var jobs, events int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM judge_jobs WHERE submission_id=$1`, sub).Scan(&jobs))
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='submission.created'`, sub).Scan(&events))
	s.Equal(1, jobs)
	s.Equal(1, events)
}
