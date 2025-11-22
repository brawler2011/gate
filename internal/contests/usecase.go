package contests

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type ContestRepo interface {
	CreateContest(ctx context.Context, c *models.CreateContestParams) error
	GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error)
	UpdateContest(ctx context.Context, c models.ContestUpdate) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*models.ContestsList, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*models.UserContestsList, error)
	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*models.ContestsList, error)
	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*models.ContestsList, error)

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (*models.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) (*models.ParticipantsList, error)

	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMemberRecord, error)
	CreateContestMember(ctx context.Context, c *models.ContestMemberParams) error
	DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error
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
	c *models.CreateContestInput,
) (uuid.UUID, error) {
	params := &models.CreateContestParams{
		Id:     uuid.New(),
		Title:  c.Title,
		UserId: c.UserId,
	}

	err := uc.contestRepo.CreateContest(ctx, params)
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create contest")
	}

	err = uc.contestRepo.CreateContestMember(ctx, &models.ContestMemberParams{
		ContestId: params.Id,
		UserId:    c.UserId,
		Role:      models.ContestRoleOwner,
	})
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create contest member")
	}

	return params.Id, nil
}

func (uc *UseCase) GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error) {
	return uc.contestRepo.GetContest(ctx, id)
}

func (uc *UseCase) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*models.ContestsList, error) {
	contestsList, err := uc.contestRepo.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list admin contests from database")
	}
	return contestsList, nil
}

func (uc *UseCase) ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*models.UserContestsList, error) {
	contestsList, err := uc.contestRepo.ListUserContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list user contests from database")
	}
	return contestsList, nil
}

func (uc *UseCase) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*models.ContestsList, error) {
	contestsList, err := uc.contestRepo.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list workshop contests from database")
	}
	return contestsList, nil
}

func (uc *UseCase) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*models.ContestsList, error) {
	contestsList, err := uc.contestRepo.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list public contests from database")
	}
	return contestsList, nil
}

func (uc *UseCase) UpdateContest(ctx context.Context, c models.ContestUpdate) error {
	err := uc.contestRepo.UpdateContest(ctx, c)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update contest")
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
	return uc.contestRepo.CreateContestMember(ctx, &models.ContestMemberParams{
		ContestId: c.ContestId,
		UserId:    c.UserId,
		Role:      models.ContestRoleParticipant,
	})
}

func (uc *UseCase) DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error {
	return uc.contestRepo.DeleteContestMember(ctx, c.ContestId, c.UserId)
}

func (uc *UseCase) ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.ParticipantsList, error) {
	return uc.contestRepo.ListContestMembers(ctx, filter)
}

func (uc *UseCase) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMemberRecord, error) {
	return uc.contestRepo.GetContestMember(ctx, c)
}
