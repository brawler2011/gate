package judging

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/events"
	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type terminalJobs struct {
	Jobs
	job      judgingmodel.Job
	released bool
}

func (j *terminalJobs) Release(context.Context, judgingmodel.Job) error {
	j.released = true
	return nil
}

type terminalStore struct {
	submission submissionsmodel.Submission
	current    bool
	update     *submissionsmodel.SubmissionUpdate
	notices    []events.Notice
}

func (s *terminalStore) CompleteJob(context.Context, judgingmodel.Job) (bool, error) {
	return s.current, nil
}
func (s *terminalStore) FailStalledJob(context.Context, judgingmodel.JobRef) (bool, error) {
	return s.current, nil
}
func (s *terminalStore) RequeueStalledJob(context.Context, judgingmodel.JobRef) (bool, error) {
	return s.current, nil
}
func (s *terminalStore) GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error) {
	return s.submission, nil
}
func (s *terminalStore) UpdateSubmission(_ context.Context, _ uuid.UUID, update *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error) {
	s.update = update
	s.submission.State = update.State
	s.submission.Revision++
	return submissionsmodel.SubmissionChange{PreviousState: submissionsmodel.Saved, Revision: s.submission.Revision}, nil
}
func (s *terminalStore) Notify(_ context.Context, n events.Notice) error {
	s.notices = append(s.notices, n)
	return nil
}
func (s *terminalStore) Within(ctx context.Context, fn func(JudgeTxStore) error) error {
	return fn(s)
}

type terminalProgress struct{}

func (terminalProgress) Notify(context.Context, events.Notice) error { return nil }

func TestJudgeMissingProblemPersistsTerminalFailure(t *testing.T) {
	for _, current := range []bool{true, false} {
		name := "current attempt"
		if !current {
			name = "stale attempt"
		}
		t.Run(name, func(t *testing.T) {
			job := judgingmodel.Job{ID: uuid.New(), SubmissionID: uuid.New(), Revision: 1}
			jobs := &terminalJobs{job: job}
			store := &terminalStore{current: current, submission: submissionsmodel.Submission{ID: job.SubmissionID, State: submissionsmodel.Saved, Revision: 1}}
			judge := NewJudgeUseCase(jobs, store, store, nil, nil, terminalProgress{})
			require.NoError(t, judge.JudgeSubmission(context.Background(), job))
			require.True(t, jobs.released)
			if !current {
				require.Nil(t, store.update)
				require.Empty(t, store.notices)
				return
			}
			require.NotNil(t, store.update)
			require.Equal(t, submissionsmodel.GotIE, store.update.State)
			require.Equal(t, []events.Notice{{Type: submissionsmodel.SubmissionEventCompleted, ID: job.SubmissionID}}, store.notices)
		})
	}
}
