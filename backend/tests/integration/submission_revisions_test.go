//go:build integration

package integration

import (
	"context"
	"strings"
	"sync"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
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
	user := s.createUser("revision_"+suffix, accessmodel.UserRoleAdmin)
	org := s.createOrganization("revision-"+suffix, "Revisions", user.Id)
	problem, err := s.client.CreateProblem(withTestUser(s.ctx, user.Id), corev1.CreateProblemParams{
		Title: "Revision", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contest := uuid.New()
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{
		ID: contest, OrganizationID: org.ID, OwnerID: &user.Id, Title: "Revision", Login: "revision-" + suffix, Visibility: contestsmodel.ContestVisibilityPublic}))
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	creation := &submissionsmodel.SubmissionCreation{ContestId: contest, ProblemId: problem.ID, UserId: user.Id, Language: submissionsmodel.Cpp, Solution: "int main(){}"}
	first, err := repo.CreateSubmission(s.ctx, creation)
	s.Require().NoError(err)
	second, err := repo.CreateSubmission(s.ctx, creation)
	s.Require().NoError(err)
	filter := submissionsmodel.SubmissionsFilter{ContestId: &contest, Page: 1, PageSize: 15}
	rows, count, err := repo.ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.Len(rows, 2)
	s.EqualValues(2, count)
	for _, row := range rows {
		s.EqualValues(1, row.Revision)
	}

	tx, err := s.dbPool.Begin(s.ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(s.ctx) }()
	_, err = repo.WithTx(tx).ResetSubmissionsState(s.ctx, submissionsmodel.RejudgeFilter{ContestID: contest, SubmissionID: &first})
	s.Require().NoError(err)
	finished := make(chan error, 1)
	go func() {
		change, updateErr := repo.UpdateSubmission(s.ctx, second, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.Accepted})
		if updateErr == nil && change.Revision != 2 {
			finished <- context.Canceled
			return
		}
		finished <- updateErr
	}()
	select {
	case err := <-finished:
		s.Require().NoError(err, "independent submission must finish before rollback")
	case <-time.After(5 * time.Second):
		s.FailNow("independent submission blocked")
	}
	_, count, err = repo.ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.EqualValues(2, count)
	s.Require().NoError(tx.Rollback(s.ctx))

	config := s.dbPool.Config()
	var updateErr error
	config.ConnConfig.Tracer = &submissionSnapshotTracer{update: func() {
		_, updateErr = repo.UpdateSubmission(s.ctx, first, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.Accepted})
	}}
	pool, err := pgxpool.NewWithConfig(s.ctx, config)
	s.Require().NoError(err)
	defer pool.Close()
	saved := submissionsmodel.Saved
	filter.State = &saved
	rows, count, err = pgstore.NewSubmissionsRepo(pool).ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.Require().NoError(updateErr)
	s.EqualValues(1, count)
	s.Require().Len(rows, 1)
	s.Equal(submissionsmodel.Saved, rows[0].State)
	s.EqualValues(1, rows[0].Revision)
	rows, count, err = repo.ListSubmissions(s.ctx, filter)
	s.Require().NoError(err)
	s.Zero(count)
	s.Empty(rows)
}
