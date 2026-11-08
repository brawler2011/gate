package submissions

import (
	"context"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	"github.com/brawler2011/gate/backend/internal/contests"
	"github.com/google/uuid"
)

type submissionTestStore struct {
	submissionCommandStore
	OutboxRepo
	contests.ScoreboardStore
}

func (submissionTestStore) ScheduleJob(_ context.Context, id uuid.UUID) (judgingmodel.Job, error) {
	return judgingmodel.Job{ID: uuid.New(), SubmissionID: id, Generation: 1}, nil
}

func (submissionTestStore) LockSubmissionTuple(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

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
	return &accessmodel.ContestPermissions{GetOwnSubmission: true, GetOtherUserSubmission: true, GetSubmissionDetails: true, ManageContest: true}, nil
}

func (submissionTestStore) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return true, nil
}

type OutboxRepo interface {
	eventdata.Writer
}

type submissionCommandStore interface {
	BlockSubmission(context.Context, uuid.UUID, *string) error
	ResetSubmissionsState(context.Context, submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error)
	CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
	ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) ([]submissionsmodel.Submission, int32, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error)
}
