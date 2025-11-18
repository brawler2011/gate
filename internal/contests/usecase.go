package contests

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type ContestRepo interface {
	CreateContest(ctx context.Context, c models.ContestCreation) (uuid.UUID, error)
	GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error)
	ListContests(ctx context.Context, filter models.ContestsFilter) (*models.ContestsList, error)
	UpdateContest(ctx context.Context, c models.ContestUpdate) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (*models.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	CreateParticipant(ctx context.Context, c models.ParticipantCreation) error
	DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error
	ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.UsersList, error)

	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMember, error)
}

type UseCase struct {
	contestRepo ContestRepo
}

func NewContestUseCase(
	contestRepo ContestRepo,
) *UseCase {
	return &UseCase{
		contestRepo: contestRepo,
	}
}

func (uc *UseCase) CreateContest(
	ctx context.Context,
	creation models.ContestCreation,
) (uuid.UUID, error) {
	const op = "UseCase.CreateContest"

	id, err := uc.contestRepo.CreateContest(ctx, creation)
	if err != nil {
		return uuid.Nil, pkg.Wrap(nil, err, op, "can't create contest")
	}

	return id, nil
}

func (uc *UseCase) GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error) {
	return uc.contestRepo.GetContest(ctx, id)
}

func (uc *UseCase) ListContests(ctx context.Context, filter models.ContestsFilter) (*models.ContestsList, error) {
	const op = "UseCase.ListContests"

	contestsList, err := uc.contestRepo.ListContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(nil, err, op, "can't list contests from database")
	}
	return contestsList, nil
}

func (uc *UseCase) UpdateContest(ctx context.Context, c models.ContestUpdate) error {
	const op = "UseCase.UpdateContest"

	err := uc.contestRepo.UpdateContest(ctx, c)
	if err != nil {
		return pkg.Wrap(nil, err, op, "can't update contest")
	}

	return nil
}

func (uc *UseCase) DeleteContest(ctx context.Context, id uuid.UUID) error {
	return uc.contestRepo.DeleteContest(ctx, id)
}

func (uc *UseCase) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
	return uc.contestRepo.CreateContestProblem(ctx, c)
}

func (uc *UseCase) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (*models.ContestProblem, error) {
	return uc.contestRepo.GetContestProblem(ctx, c)
}

func (uc *UseCase) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error) {
	return uc.contestRepo.GetContestProblems(ctx, contestId)
}

func (uc *UseCase) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	return uc.contestRepo.DeleteContestProblem(ctx, c)
}

func (uc *UseCase) CreateParticipant(ctx context.Context, c models.ParticipantCreation) error {
	return uc.contestRepo.CreateParticipant(ctx, c)
}

func (uc *UseCase) DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error {
	return uc.contestRepo.DeleteParticipant(ctx, c)
}

func (uc *UseCase) ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.UsersList, error) {
	return uc.contestRepo.ListParticipants(ctx, filter)
}

func (uc *UseCase) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMember, error) {
	return uc.contestRepo.GetContestMember(ctx, c)
}
