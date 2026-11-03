package core_test

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type ContestsUC interface {
	CreateContest(ctx context.Context, c *contestsmodel.CreateContestInput) (uuid.UUID, error)
	GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error)
	GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error)
	UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	CreateParticipant(ctx context.Context, c contestsmodel.ParticipantCreation) error
	GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error)
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteParticipant(ctx context.Context, c contestsmodel.ParticipantDeletion) error

	CreateContestTeam(ctx context.Context, contestID, teamID, requestUserID uuid.UUID, role contestsmodel.ContestRole) error
	UpdateContestTeamRole(ctx context.Context, contestID, teamID, requestUserID uuid.UUID, role contestsmodel.ContestRole) error
	DeleteContestTeam(ctx context.Context, contestID, teamID, requestUserID uuid.UUID) error

	CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestProblem, error)
	UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error
	DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error
	ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error

	ProcessSubmissionResult(ctx context.Context, submission *submissionsmodel.Submission) error
	GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error)
	GetContestScoreboardUser(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error)

	BlockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string, operatorID uuid.UUID) error
	UnblockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, rejudgeSubmissions bool) error
	GetProblemBlockStatusForUser(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error)

	// Contest Join Requests
	CreateJoinRequest(ctx context.Context, orgLogin, contestLogin string, userID uuid.UUID, message *string) (*contestsmodel.ContestJoinRequest, bool, error)
	ListJoinRequests(ctx context.Context, orgLogin, contestLogin string, requestUserID uuid.UUID) ([]contestsmodel.ContestJoinRequest, error)
	CancelJoinRequest(ctx context.Context, orgLogin, contestLogin string, requestUserID uuid.UUID) error
	ApproveJoinRequest(ctx context.Context, orgLogin, contestLogin string, requestID, reviewerID uuid.UUID) error
	RejectJoinRequest(ctx context.Context, orgLogin, contestLogin string, requestID, reviewerID uuid.UUID) error
	GetMyPendingJoinRequest(ctx context.Context, orgLogin, contestLogin string, userID uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
}

type PermissionsUC interface {
	GetContestPermissions(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*accessmodel.ContestPermissions, error)
	GetProblemPermissions(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*accessmodel.ProblemPermissions, error)
	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action accessmodel.ContestAction) (bool, error)
	HasProblemPermission(ctx context.Context, problemID uuid.UUID, userID uuid.UUID, action accessmodel.ProblemAction) (bool, error)
	HasOrganizationPermission(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action accessmodel.OrgAction) (bool, error)
	GetEffectiveContestRole(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*contestsmodel.ContestRole, contestsmodel.ContestPermissionMask, error)
}

type ProblemsUC interface {
	CreateProblem(ctx context.Context, input *problemsmodel.CreateProblemInput) (uuid.UUID, error)
	CreateProblemTests(ctx context.Context, problemId uuid.UUID, tests []problemsmodel.ProblemTest) error
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error)
	GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]problemsmodel.ProblemTest, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error
	UploadProblemTests(ctx context.Context, problemId uuid.UUID, zipData []byte) error

	AddProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID) error

	CreateProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error
	UpdateProblemMemberRole(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID) error
}
