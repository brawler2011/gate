package submissions

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	"github.com/brawler2011/gate/backend/internal/contests"
	"github.com/google/uuid"
)

type submissionTestStore struct {
	SubmissionsRepo
	OutboxRepo
	contests.ScoreboardStore
}

func (submissionTestStore) ScheduleJob(_ context.Context, id uuid.UUID) (judgingmodel.Job, error) {
	return judgingmodel.Job{ID: uuid.New(), SubmissionID: id, Generation: 1}, nil
}

func (submissionTestStore) LockContest(context.Context, uuid.UUID) error { return nil }

func (submissionTestStore) GetContestProblem(context.Context, contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error) {
	return contestsmodel.ContestProblem{}, nil
}
func (submissionTestStore) GetContestUserProblemBlock(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error) {
	return nil, nil
}

type allowSubmissionPermissions struct{}

func (allowSubmissionPermissions) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return true, nil
}
func (allowSubmissionPermissions) GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*accessmodel.ContestPermissions, error) {
	return &accessmodel.ContestPermissions{GetOtherUserSubmission: true, GetSubmissionDetails: true, ManageContest: true}, nil
}
