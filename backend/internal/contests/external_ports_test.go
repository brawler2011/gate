package contests_test

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type ContestsRepo interface {
	ResolveContestProblemPackage(context.Context, uuid.UUID, uuid.UUID, bool) (access.PackageContext, error)
	GetContestScoreboardSubmissions(context.Context, uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error)
	GetUserScoreboardSubmissions(context.Context, uuid.UUID, uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error)
	CreateContest(ctx context.Context, params *contestsmodel.CreateContestParams) error
	GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error)
	GetContestByLogin(ctx context.Context, orgID uuid.UUID, login string) (contestsmodel.Contest, error)
	GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error)
	UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	CreateContestMember(ctx context.Context, c *contestsmodel.CreateContestMemberParams) error
	GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error)
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error

	CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error)
	UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error
	DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error
	ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error

	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestProblem, error)
	GetContestTeams(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestTeam, error)
	CreateContestTeam(ctx context.Context, contestID, teamID uuid.UUID, role contestsmodel.ContestRole) error
	UpdateContestTeamRole(ctx context.Context, contestID, teamID uuid.UUID, role contestsmodel.ContestRole) error
	DeleteContestTeam(ctx context.Context, contestID, teamID uuid.UUID) error

	UpsertContestProblemResult(ctx context.Context, params *contestsmodel.UpsertContestProblemResultParams) error
	GetContestProblemResult(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestProblemResult, error)
	GetContestScoreboardFromStandings(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ContestProblemResult, map[uuid.UUID]string, error)
	GetSubmissionsForScoreboard(ctx context.Context, contestID, userID, problemID uuid.UUID) ([]contestsmodel.SubmissionForScoreboard, error)

	CreateContestUserProblemBlock(ctx context.Context, params *contestsmodel.CreateContestUserProblemBlockParams) error
	DeleteContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) error
	GetContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error)
	ListContestUserProblemBlocks(ctx context.Context, contestID uuid.UUID, userID *uuid.UUID) ([]contestsmodel.ContestUserProblemBlock, error)
	ListUserContestMemberships(ctx context.Context, userID uuid.UUID) ([]contestsmodel.UserContestMembership, error)
	AddContestMemberIfNotExists(ctx context.Context, contestID, userID uuid.UUID, role string) error

	// Contest Join Requests
	CreateContestJoinRequest(ctx context.Context, input *contestsmodel.CreateContestJoinRequestInput) (*contestsmodel.ContestJoinRequest, error)
	GetContestJoinRequestByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	GetPendingContestJoinRequest(ctx context.Context, contestID, userID uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	ListContestJoinRequests(ctx context.Context, contestID uuid.UUID, status *string) ([]contestsmodel.ContestJoinRequest, error)
	ListUserContestJoinRequests(ctx context.Context, userID uuid.UUID) ([]contestsmodel.ContestJoinRequest, error)
	UpdateContestJoinRequestStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, reviewedBy *uuid.UUID) error
}

type OutboxRepo interface {
	eventdata.Writer
	DeleteOldEvents(ctx context.Context, retentionDays int32) error
	PickEvents(ctx context.Context, limit int32, timeoutSec int32) ([]eventdata.OutboxEvent, error)
	CreateEvent(ctx context.Context, event *eventdata.CreateOutboxEventParams) error
	MarkAsCompleted(ctx context.Context, id, leaseToken uuid.UUID) error
	MarkAsFailed(ctx context.Context, id, leaseToken uuid.UUID, errorMsg string, permanent bool) error
}

type ProblemsUC interface {
	CreateProblem(ctx context.Context, input *problemsmodel.CreateProblemInput) (uuid.UUID, error)
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error

	AddProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID) error

	CreateProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error
	UpdateProblemMemberRole(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID) error
}

type SubmissionsRepo interface {
	CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
	ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) ([]submissionsmodel.Submission, int32, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *submissionsmodel.SubmissionUpdate) (submissionsmodel.SubmissionChange, error)
	BlockSubmission(ctx context.Context, id uuid.UUID, reason *string) error
	BlockUserProblemSubmissions(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string) error
	ResetSubmissionsState(ctx context.Context, filter submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error)
}
