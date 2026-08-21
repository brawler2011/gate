package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ContestsRepo interface {
	CreateContest(ctx context.Context, params *models.CreateContestParams) error
	GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error)
	GetContestByLogin(ctx context.Context, orgID uuid.UUID, login string) (models.Contest, error)
	GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (models.Contest, error)
	UpdateContest(ctx context.Context, c models.ContestUpdateParams) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]models.Contest, int32, error)
	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]models.Contest, int32, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]models.Contest, int32, error)
	ListOrganizationContests(ctx context.Context, orgID uuid.UUID, search string, visibility string, page, pageSize int32) ([]models.Contest, int32, error)

	CreateContestMember(ctx context.Context, c *models.CreateContestMemberParams) error
	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error)
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error

	ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) ([]models.ContestMember, int32, error)

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error)
	UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error)
	GetContestTeams(ctx context.Context, contestId uuid.UUID) ([]models.ContestTeam, error)
	CreateContestTeam(ctx context.Context, contestID, teamID uuid.UUID, role models.ContestRole) error
	UpdateContestTeamRole(ctx context.Context, contestID, teamID uuid.UUID, role models.ContestRole) error
	DeleteContestTeam(ctx context.Context, contestID, teamID uuid.UUID) error

	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]models.Contest, int32, error)
	ListDashboardContests(ctx context.Context, userID uuid.UUID, limit int32) ([]models.DashboardContest, error)

	UpsertContestProblemResult(ctx context.Context, params *models.UpsertContestProblemResultParams) error
	GetContestProblemResult(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestProblemResult, error)
	GetContestScoreboardFromStandings(ctx context.Context, contestID uuid.UUID) ([]models.ContestProblemResult, map[uuid.UUID]string, error)
	GetSubmissionsForScoreboard(ctx context.Context, contestID, userID, problemID uuid.UUID) ([]models.SubmissionForScoreboard, error)

	CreateContestUserProblemBlock(ctx context.Context, params *models.CreateContestUserProblemBlockParams) error
	DeleteContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) error
	GetContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestUserProblemBlock, error)
	ListContestUserProblemBlocks(ctx context.Context, contestID uuid.UUID, userID *uuid.UUID) ([]models.ContestUserProblemBlock, error)
	WithTx(tx pgx.Tx) ContestsRepo
}

type ContestsUC interface {
	CreateContest(ctx context.Context, c *models.CreateContestInput) (uuid.UUID, error)
	GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error)
	GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (models.Contest, error)
	UpdateContest(ctx context.Context, c models.ContestUpdateInput) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*models.ContestsList, error)
	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*models.ContestsList, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*models.ContestsList, error)
	ListOrganizationContests(ctx context.Context, orgID uuid.UUID, search string, visibility string, page, pageSize int32) (*models.ContestsList, error)
	ListDashboardContests(ctx context.Context, userID uuid.UUID, limit int32) ([]models.DashboardContest, error)
	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*models.ContestsList, error)

	CreateParticipant(ctx context.Context, c models.ParticipantCreation) error
	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error)
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error
	ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.ContestMembersList, error)

	CreateContestTeam(ctx context.Context, contestID, teamID, requestUserID uuid.UUID, role models.ContestRole) error
	GetContestTeams(ctx context.Context, contestID, requestUserID uuid.UUID) ([]models.ContestTeam, error)
	UpdateContestTeamRole(ctx context.Context, contestID, teamID, requestUserID uuid.UUID, role models.ContestRole) error
	DeleteContestTeam(ctx context.Context, contestID, teamID, requestUserID uuid.UUID) error

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error)
	UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	ProcessSubmissionResult(ctx context.Context, submission *models.Submission) error
	GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*models.ScoreboardResponse, error)

	BlockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string, operatorID uuid.UUID) error
	UnblockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, rejudgeSubmissions bool) error
	GetProblemBlockStatusForUser(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestUserProblemBlock, error)
}

