//go:build integration

package integration

import (
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
)

func (s *IntegrationTestSuite) TestClarificationAnswerAndAnnouncementRollBackTogether() {
	_, contestID, _, ownerID := s.newJobSubmission()
	repo := pgstore.NewClarificationsRepo(s.dbPool)
	question, err := repo.CreateClarification(s.ctx, &contestsmodel.CreateContestClarificationInput{ContestID: contestID, UserID: ownerID, Question: "Question"})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `CREATE FUNCTION reject_announcement_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.event_type = 'contest.announcement_created' THEN RAISE EXCEPTION 'injected announcement event failure'; END IF;
 RETURN NEW; END $$;
 CREATE TRIGGER reject_announcement_event BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION reject_announcement_event()`)
	s.Require().NoError(err)
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, `DROP TRIGGER reject_announcement_event ON outbox_events; DROP FUNCTION reject_announcement_event()`)
		s.Require().NoError(err)
	}()
	uc := contests.NewClarificationsUseCase(pgstore.NewContestEventsTransactions(s.dbPool))
	ctx := accessmodel.WithUser(s.ctx, accessmodel.User{Id: ownerID, Role: accessmodel.UserRoleUser})
	_, err = uc.AnswerClarification(ctx, &contestsmodel.AnswerContestClarificationInput{ContestID: contestID, ClarificationID: question.ID, AnsweredBy: ownerID, Answer: "Answer", PublishAsAnnouncement: true})
	s.Require().Error(err)
	stored, err := repo.GetClarificationByID(s.ctx, question.ID)
	s.Require().NoError(err)
	s.Equal(contestsmodel.ClarificationStatusPending, stored.Status)
	s.Nil(stored.Answer)
	var count int
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM contest_announcements WHERE contest_id=$1`, contestID).Scan(&count))
	s.Zero(count)
	s.Require().NoError(s.dbPool.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1`, contestID).Scan(&count))
	s.Zero(count, "the first event must roll back when the subsequent announcement event fails")
}
