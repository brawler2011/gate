package submissions

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
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
	submissionCommandStore
	submission submissionsmodel.Submission
}

func (r *diagnosticSubmissionsRepo) CreateSubmission(context.Context, *submissionsmodel.SubmissionCreation) (uuid.UUID, error) {
	return r.submission.ID, nil
}

func (r *diagnosticSubmissionsRepo) GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error) {
	return r.submission, nil
}

func (r *diagnosticSubmissionsRepo) ResetSubmissionsState(context.Context, submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error) {
	return []submissionsmodel.SubmissionReset{{ID: r.submission.ID, PreviousState: r.submission.State}}, nil
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
			notices := &noticeLog{}
			viewerID := uuid.New()
			loader := accesstest.Loader{Contests: map[uuid.UUID]access.ContestFacts{contestID: {Contest: contestsmodel.Contest{ID: contestID}, Now: time.Now()}}}
			loader.Grant(contestID, viewerID, string(accessmodel.ContestModerator))
			uc := NewSubmissionsUseCase(repo, diagnosticTransactor{commitErr, &buf, t, submissionTestStore{Loader: loader, submissionCommandStore: repo, Writer: notices}}, loader, OutputsStore{})
			ctx := accessmodel.WithUser(requestid.With(context.Background(), "request-123"), accessmodel.User{Id: viewerID, Role: accessmodel.UserRoleUser})
			var err error
			if rejudge {
				_, err = uc.RejudgeSubmissions(ctx, submissionsmodel.RejudgeFilter{ContestID: contestID})
			} else {
				_, err = uc.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{ContestId: contestID, UserId: viewerID})
			}
			require.Equal(t, commitErr, err)
			require.Equal(t, []eventdata.Notice{{Type: submissionsmodel.SubmissionEventCreated, ID: submissionID}}, notices.notices)
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

func (r *diagnosticSubmissionsRepo) ScheduleJob(_ context.Context, id uuid.UUID) (judgingmodel.Job, error) {
	return judgingmodel.Job{ID: uuid.New(), SubmissionID: id, Revision: 1}, nil
}
