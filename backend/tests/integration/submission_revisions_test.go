//go:build integration

package integration

import (
	"context"
	"strings"
	"sync"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type submissionSnapshotTracer struct {
	once   sync.Once
	update func()
}
type countSnapshotKey struct{}

func (t *submissionSnapshotTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, countSnapshotKey{}, strings.Contains(data.SQL, "-- name: CountSubmissions"))
}
func (t *submissionSnapshotTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(countSnapshotKey{}) == true {
		t.once.Do(t.update)
	}
}

func (s *IntegrationTestSuite) TestSubmissionRevisionSnapshot() {
	suffix := uuid.NewString()[:8]
	user := s.createUser("revision_"+suffix, models.UserRoleAdmin)
	org := s.createOrganization("revision-"+suffix, "Revisions", user.Id)
	problem, err := s.client.CreateProblem(withTestUser(s.ctx, user.Id), corev1.CreateProblemParams{
		Title: "Revision", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contest := uuid.New()
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
		ID: contest, OrganizationID: org.ID, OwnerID: &user.Id, Title: "Revision", Login: "revision-" + suffix, Visibility: models.ContestVisibilityPublic}))
	repo := pg.NewSubmissionsRepo(s.dbPool)
	creation := &models.SubmissionCreation{ContestId: contest, ProblemId: problem.ID, UserId: user.Id, Language: models.Cpp, Solution: "int main(){}"}
	first, err := repo.CreateSubmission(s.ctx, creation)
	s.Require().NoError(err)
	second, err := repo.CreateSubmission(s.ctx, creation)
	s.Require().NoError(err)
	filter := models.SubmissionsFilter{ContestId: &contest, Page: 1, PageSize: 15}
	rows, count, revision, err := repo.ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.Len(rows, 2)
	s.EqualValues(2, count)
	s.EqualValues(2, revision)

	tx, err := s.dbPool.Begin(s.ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(s.ctx) }()
	_, err = repo.WithTx(tx).ResetSubmissionsState(s.ctx, models.RejudgeFilter{ContestID: contest, SubmissionID: &first})
	s.Require().NoError(err)
	finished := make(chan error, 1)
	go func() {
		change, updateErr := repo.UpdateSubmission(s.ctx, second, &models.SubmissionUpdate{State: models.Accepted})
		if updateErr == nil && change.Revision != 3 {
			finished <- context.Canceled
			return
		}
		finished <- updateErr
	}()
	select {
	case err := <-finished:
		s.FailNow("writer passed an uncommitted contest revision", "%v", err)
	case <-time.After(50 * time.Millisecond):
	}
	_, count, revision, err = repo.ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.EqualValues(2, count)
	s.EqualValues(2, revision)
	s.Require().NoError(tx.Rollback(s.ctx))
	s.Require().NoError(<-finished)

	config := s.dbPool.Config()
	var updateErr error
	config.ConnConfig.Tracer = &submissionSnapshotTracer{update: func() {
		_, updateErr = repo.UpdateSubmission(s.ctx, first, &models.SubmissionUpdate{State: models.Accepted})
	}}
	pool, err := pgxpool.NewWithConfig(s.ctx, config)
	s.Require().NoError(err)
	defer pool.Close()
	saved := models.Saved
	filter.State = &saved
	rows, count, revision, err = pg.NewSubmissionsRepo(pool).ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.Require().NoError(updateErr)
	s.EqualValues(3, revision)
	s.EqualValues(1, count)
	s.Require().Len(rows, 1)
	s.Equal(models.Saved, rows[0].State)
	rows, count, revision, err = repo.ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.EqualValues(4, revision)
	s.Zero(count)
	s.Empty(rows)
}
