package submissions

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/contests"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	"github.com/google/uuid"
)

type submissionTestStore struct {
	SubmissionsRepo
	OutboxRepo
	contests.ScoreboardStore
}

func (submissionTestStore) ScheduleJob(_ context.Context, id uuid.UUID) (judging.Job, error) {
	return judging.Job{ID: uuid.New(), SubmissionID: id, Generation: 1}, nil
}

func (submissionTestStore) LockContest(context.Context, uuid.UUID) error { return nil }
