package submissions

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestRejudgeSubmissions_NilContestID(t *testing.T) {
	uc := &SubmissionsUseCase{}
	_, err := uc.RejudgeSubmissions(context.Background(), submissionsmodel.RejudgeFilter{
		ContestID: uuid.Nil,
	})
	if err == nil {
		t.Fatal("expected error for nil contest ID, got nil")
	}
}

type diagnosticSubmissionsRepo struct {
	SubmissionsRepo
	submission submissionsmodel.Submission
}

func (r *diagnosticSubmissionsRepo) WithTx(pgx.Tx) SubmissionsRepo { return r }

func (r *diagnosticSubmissionsRepo) CreateSubmission(context.Context, *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	return r.submission.ID, nil
}

func (r *diagnosticSubmissionsRepo) GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error) {
	return r.submission, nil
}

func (r *diagnosticSubmissionsRepo) ResetSubmissionsState(context.Context, submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error) {
	return []submissionsmodel.SubmissionReset{{ID: r.submission.ID, PreviousState: r.submission.State}}, nil
}

type diagnosticOutboxRepo struct {
	OutboxRepo
	event *eventdata.CreateOutboxEventParams
}

func (r *diagnosticOutboxRepo) WithTx(pgx.Tx) OutboxRepo { return r }

func (r *diagnosticOutboxRepo) CreateEvent(_ context.Context, event *eventdata.CreateOutboxEventParams) error {
	r.event = event
	return nil
}

type diagnosticContests struct{ ContestsUC }

func (diagnosticContests) GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error) {
	return contestsmodel.Contest{}, nil
}

func (diagnosticContests) GetContestProblem(context.Context, contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error) {
	return contestsmodel.ContestProblem{}, nil
}

func (diagnosticContests) GetProblemBlockStatusForUser(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error) {
	return nil, nil
}

type diagnosticProblems struct{ ProblemsUC }

func (diagnosticProblems) GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error) {
	return problemsmodel.Problem{}, nil
}

type diagnosticTransactor struct {
	commitErr error
	buf       *bytes.Buffer
	t         *testing.T
	store     SubmissionTxStore
}

func (tx diagnosticTransactor) Within(ctx context.Context, fn func(SubmissionTxStore) error) error {
	if err := fn(tx.store); err != nil {
		return err
	}
	require.Empty(tx.t, tx.buf.String(), "no acceptance log may precede commit")
	return tx.commitErr
}

func TestSubmissionDiagnosticEventsOnlyAfterCommit(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)
	for _, commitErr := range []error{nil, errors.New("commit failed")} {
		for _, rejudge := range []bool{false, true} {
			var buf bytes.Buffer
			slog.SetDefault(logging.New(&buf))
			contestID := uuid.New()
			submissionID := uuid.New()
			repo := &diagnosticSubmissionsRepo{submission: submissionsmodel.Submission{ID: submissionID, ContestID: &contestID, CreatedAt: time.Now().Add(-24 * time.Hour)}}
			outbox := &diagnosticOutboxRepo{}
			uc := NewSubmissionsUseCase(repo, diagnosticContests{}, diagnosticProblems{}, diagnosticTransactor{commitErr, &buf, t, submissionTestStore{SubmissionsRepo: repo, OutboxRepo: outbox}})
			ctx := requestid.With(context.Background(), "request-123")
			var err error
			if rejudge {
				_, err = uc.RejudgeSubmissions(ctx, submissionsmodel.RejudgeFilter{ContestID: contestID})
			} else {
				_, err = uc.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{ContestId: contestID})
			}
			require.Equal(t, commitErr, err)
			require.NotNil(t, outbox.event)
			require.Equal(t, "request-123", outbox.event.Headers[requestid.Header])
			if commitErr != nil {
				require.Empty(t, buf.String())
			} else {
				event := "submission_accepted"
				if rejudge {
					event = "submission_rejudge_requested"
				}
				require.Contains(t, buf.String(), event)
				require.Contains(t, buf.String(), submissionID.String())
				require.Contains(t, buf.String(), "request-123")
			}
		}
	}
}

func (r *diagnosticOutboxRepo) Record(ctx context.Context, event eventdata.Event) error {
	return recordTestEvent(ctx, event, r.CreateEvent)
}

func (r *diagnosticSubmissionsRepo) ScheduleJob(_ context.Context, id uuid.UUID) (judging.Job, error) {
	return judging.Job{ID: uuid.New(), SubmissionID: id, Generation: 1}, nil
}
