package usecase

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

type ContestsUseCase struct {
	contestRepo interfaces.ContestsRepo
}

func NewContestsUseCase(
	contestRepo interfaces.ContestsRepo,
) *ContestsUseCase {
	return &ContestsUseCase{
		contestRepo: contestRepo,
	}
}

func (uc *ContestsUseCase) CreateContest(
	ctx context.Context,
	c *models.CreateContestInput,
) (uuid.UUID, error) {
	params := &models.CreateContestParams{
		ID:             uuid.New(),
		OrganizationID: c.OrganizationID,
		OwnerID:        c.OwnerID,
		Visibility:     c.Visibility,
		Title:          c.Title,
		ShortName:      c.ShortName,
		Description:    c.Description,
		Settings:       c.Settings,
		AccessPolicy:   c.AccessPolicy,
		StartTime:      c.StartTime,
		EndTime:        c.EndTime,
	}

	err := uc.contestRepo.CreateContest(ctx, params)
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create contest")
	}

	// If an owner ID was provided, add them as a contest member with owner role
	if c.OwnerID != nil {
		err = uc.contestRepo.CreateContestMember(ctx, &models.CreateContestMemberParams{
			ContestId: params.ID,
			UserId:    *c.OwnerID,
			Role:      models.ContestRoleOwner,
		})
		if err != nil {
			return uuid.Nil, pkg.Wrap(err, nil, "can't create contest member")
		}
	}

	return params.ID, nil
}

func (uc *ContestsUseCase) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	contest, err := uc.contestRepo.GetContest(ctx, id)
	if err != nil {
		return models.Contest{}, err
	}
	return contest, nil
}

func (uc *ContestsUseCase) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list admin contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListUserContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list user contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list workshop contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list public contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) UpdateContest(ctx context.Context, c models.ContestUpdateInput) error {
	params := models.ContestUpdateParams{
		ID:           c.ID,
		Title:        c.Title,
		Description:  c.Description,
		Visibility:   c.Visibility,
		Settings:     c.Settings,
		AccessPolicy: c.AccessPolicy,
		StartTime:    c.StartTime,
		EndTime:      c.EndTime,
		OwnerID:      c.OwnerID,
	}

	return uc.contestRepo.UpdateContest(ctx, params)
}

func (uc *ContestsUseCase) DeleteContest(ctx context.Context, id uuid.UUID) error {
	return uc.contestRepo.DeleteContest(ctx, id)
}

func (uc *ContestsUseCase) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
	return uc.contestRepo.CreateContestProblem(ctx, c)
}

func (uc *ContestsUseCase) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error) {
	return uc.contestRepo.GetContestProblem(ctx, c)
}

func (uc *ContestsUseCase) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error) {
	return uc.contestRepo.GetContestProblems(ctx, contestId)
}

func (uc *ContestsUseCase) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	return uc.contestRepo.DeleteContestProblem(ctx, c)
}

func (uc *ContestsUseCase) CreateParticipant(ctx context.Context, c models.ParticipantCreation) error {
	return uc.contestRepo.CreateContestMember(ctx, &models.CreateContestMemberParams{
		ContestId: c.ContestId,
		UserId:    c.UserId,
		Role:      models.ContestRoleParticipant,
	})
}

func (uc *ContestsUseCase) DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error {
	return uc.contestRepo.DeleteContestMember(ctx, c.UserId, c.ContestId)
}

func (uc *ContestsUseCase) ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.ContestMembersList, error) {
	members, total, err := uc.contestRepo.ListContestMembers(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &models.ContestMembersList{
		Members:    members,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error) {
	return uc.contestRepo.GetContestMember(ctx, c)
}

func (uc *ContestsUseCase) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error {
	if role != models.ContestRoleOwner && role != models.ContestRoleModerator && role != models.ContestRoleParticipant {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	currentMember, err := uc.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestId,
		UserId:    userId,
	})
	if err != nil {
		return pkg.Wrap(err, nil, "can't get contest member")
	}

	if currentMember.Role == models.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role from owner")
	}

	if role == models.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role to owner")
	}

	err = uc.contestRepo.UpdateContestMember(ctx, contestId, userId, role)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update contest member")
	}

	return nil
}
