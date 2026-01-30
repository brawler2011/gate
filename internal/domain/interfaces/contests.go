package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

type ContestsRepo interface {
	CreateContest(ctx context.Context, params *models.CreateContestParams) error
	GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error)
	UpdateContest(ctx context.Context, c models.ContestUpdateParams) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]models.Contest, int32, error)
	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]models.Contest, int32, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]models.Contest, int32, error)

	CreateContestMember(ctx context.Context, c *models.CreateContestMemberParams) error
	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error)
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error

	ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) ([]models.ContestMember, int32, error)

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error)
	GetContestTeams(ctx context.Context, contestId uuid.UUID) ([]models.ContestTeam, error)

	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]models.Contest, int32, error)
}

type ContestsUC interface {
	CreateContest(ctx context.Context, c *models.CreateContestInput) (uuid.UUID, error)
	GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error)
	UpdateContest(ctx context.Context, c models.ContestUpdateInput) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*models.ContestsList, error)
	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*models.ContestsList, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*models.ContestsList, error)
	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*models.ContestsList, error)

	CreateParticipant(ctx context.Context, c models.ParticipantCreation) error
	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error)
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error
	ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.ContestMembersList, error)

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error
}
