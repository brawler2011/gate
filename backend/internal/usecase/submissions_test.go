package usecase

import (
	"bytes"
	"context"
	"errors"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func TestRejudgeSubmissions_NilContestID(t *testing.T) {
	uc := &SubmissionsUseCase{}
	_, err := uc.RejudgeSubmissions(context.Background(), models.RejudgeFilter{
		ContestID: uuid.Nil,
	})
	if err == nil {
		t.Fatal("expected error for nil contest ID, got nil")
	}
}

type diagnosticSubmissionsRepo struct {
	interfaces.SubmissionsRepo
	submission models.Submission
}

func (r *diagnosticSubmissionsRepo) WithTx(pgx.Tx) interfaces.SubmissionsRepo { return r }
func (r *diagnosticSubmissionsRepo) CreateSubmission(context.Context, *models.SubmissionCreation) (uuid.UUID, error) {
	return r.submission.ID, nil
}
func (r *diagnosticSubmissionsRepo) GetSubmission(context.Context, uuid.UUID) (models.Submission, error) {
	return r.submission, nil
}
func (r *diagnosticSubmissionsRepo) ResetSubmissionsState(context.Context, models.RejudgeFilter) ([]uuid.UUID, error) {
	return []uuid.UUID{r.submission.ID}, nil
}

type diagnosticOutboxRepo struct {
	interfaces.OutboxRepo
	event *models.CreateOutboxEventParams
}

func (r *diagnosticOutboxRepo) WithTx(pgx.Tx) interfaces.OutboxRepo { return r }
func (r *diagnosticOutboxRepo) CreateEvent(_ context.Context, event *models.CreateOutboxEventParams) error {
	r.event = event
	return nil
}

type diagnosticContests struct{ interfaces.ContestsUC }

func (diagnosticContests) GetContest(context.Context, uuid.UUID) (models.Contest, error) {
	return models.Contest{}, nil
}
func (diagnosticContests) GetContestProblem(context.Context, models.ContestProblemGet) (models.ContestProblem, error) {
	return models.ContestProblem{}, nil
}
func (diagnosticContests) GetProblemBlockStatusForUser(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*models.ContestUserProblemBlock, error) {
	return nil, nil
}

type diagnosticProblems struct{ interfaces.ProblemsUC }

func (diagnosticProblems) GetProblemById(context.Context, uuid.UUID) (models.Problem, error) {
	return models.Problem{}, nil
}

type diagnosticTransactor struct {
	commitErr error
	buf       *bytes.Buffer
	t         *testing.T
}

func (tx diagnosticTransactor) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	if err := fn(ctx, nil); err != nil {
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
			repo := &diagnosticSubmissionsRepo{submission: models.Submission{ID: submissionID, ContestID: &contestID, CreatedAt: time.Now().Add(-24 * time.Hour)}}
			outbox := &diagnosticOutboxRepo{}
			uc := NewSubmissionsUseCase(repo, diagnosticContests{}, diagnosticProblems{}, outbox, diagnosticTransactor{commitErr, &buf, t})
			ctx := requestid.With(context.Background(), "request-123")
			var err error
			if rejudge {
				_, err = uc.RejudgeSubmissions(ctx, models.RejudgeFilter{ContestID: contestID})
			} else {
				_, err = uc.CreateSubmission(ctx, &models.SubmissionCreation{ContestId: contestID})
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
