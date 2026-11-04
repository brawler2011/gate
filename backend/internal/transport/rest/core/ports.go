package core

import (
	"context"

	problems "github.com/brawler2011/gate/backend/internal/problems"

	contests "github.com/brawler2011/gate/backend/internal/contests"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type AuthUC interface {
	Register(ctx context.Context, username, email, password string) (accessmodel.User, error)
	Login(ctx context.Context, identifier, password string) (accessmodel.User, uuid.UUID, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error

	VerifyEmail(ctx context.Context, token string) (accessmodel.User, uuid.UUID, error)
	ResendVerificationEmail(ctx context.Context, identifier string) error
	ForgotPassword(ctx context.Context, identifier string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (uuid.UUID, error)
	RequestEmailChange(ctx context.Context, userID uuid.UUID, password, newEmail string) error
	ConfirmEmailChange(ctx context.Context, token string) error
}

type ContestsUC interface {
	CreateContest(ctx context.Context, c *contestsmodel.CreateContestInput) (uuid.UUID, error)
	UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	CreateParticipant(ctx context.Context, c contestsmodel.ParticipantCreation) error

	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteParticipant(ctx context.Context, c contestsmodel.ParticipantDeletion) error

	CreateContestTeam(ctx context.Context, contestID, teamID, requestUserID uuid.UUID, role contestsmodel.ContestRole) error
	UpdateContestTeamRole(ctx context.Context, contestID, teamID, requestUserID uuid.UUID, role contestsmodel.ContestRole) error
	DeleteContestTeam(ctx context.Context, contestID, teamID, requestUserID uuid.UUID) error

	CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error

	DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error
	ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error

	GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error)

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

type DraftsUC interface {
	CreateDraft(ctx context.Context, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error)
	DeleteDraft(ctx context.Context, contestID, draftID uuid.UUID) error
}

type NotificationsUC interface {
	MarkAsRead(ctx context.Context, id, userID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
}

type OrganizationsUC interface {
	CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error)

	GetOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID) (*organizationsmodel.Organization, error)
	ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter, userID uuid.UUID) (*organizationsmodel.OrganizationList, error)

	UpdateOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID, input *organizationsmodel.UpdateOrganizationInput) error

	DeleteOrganizationByLogin(ctx context.Context, login string, userID uuid.UUID) error

	// Member management

	AddMemberByLogin(ctx context.Context, login string, input *organizationsmodel.AddOrganizationMemberInput, requestUserID uuid.UUID) error
	ListMembers(ctx context.Context, orgID, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationMember, error)
	ListMembersByLogin(ctx context.Context, login string, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationMember, error)

	RemoveMemberByLogin(ctx context.Context, login string, userID, requestUserID uuid.UUID) error

	BatchCreateUsers(ctx context.Context, input organizationsmodel.BatchCreateOrganizationUsersInput, requestUserID uuid.UUID) (*organizationsmodel.BatchCreateOrganizationUsersResult, error)

	// Invitations
	InviteMember(ctx context.Context, orgLogin string, targetUserID uuid.UUID, role organizationsmodel.OrganizationRole, inviterID uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	ListInvitations(ctx context.Context, orgLogin string, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationInvitation, error)
	CancelInvitation(ctx context.Context, orgLogin string, invitationID, requestUserID uuid.UUID) error
	AcceptInvitation(ctx context.Context, invitationID, requestUserID uuid.UUID) error
	DeclineInvitation(ctx context.Context, invitationID, requestUserID uuid.UUID) error

	// Join Requests
	CreateJoinRequest(ctx context.Context, orgLogin string, userID uuid.UUID, message *string) (*organizationsmodel.OrganizationJoinRequest, bool, error)
	ListJoinRequests(ctx context.Context, orgLogin string, requestUserID uuid.UUID) ([]organizationsmodel.OrganizationJoinRequest, error)
	CancelJoinRequest(ctx context.Context, orgLogin string, requestUserID uuid.UUID) error
	ApproveJoinRequest(ctx context.Context, orgLogin string, requestID, reviewerID uuid.UUID, role organizationsmodel.OrganizationRole) error
	RejectJoinRequest(ctx context.Context, orgLogin string, requestID, reviewerID uuid.UUID) error
	GetMyPendingJoinRequest(ctx context.Context, orgLogin string, userID uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
}

type PermissionsUC interface {
	GetContestPermissions(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*accessmodel.ContestPermissions, error)

	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action accessmodel.ContestAction) (bool, error)

	GetEffectiveContestRole(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*contestsmodel.ContestRole, contestsmodel.ContestPermissionMask, error)
}

type ProblemsUC interface {
	GetProblemDetails(context.Context, uuid.UUID) (*problemsmodel.ProblemDetails, error)
	CreateProblem(ctx context.Context, input *problemsmodel.CreateProblemInput) (uuid.UUID, error)

	DeleteProblem(ctx context.Context, id uuid.UUID) error

	UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error

	AddProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID, requestUserID uuid.UUID, permission problemsmodel.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID, requestUserID uuid.UUID) error

	CreateProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error
	UpdateProblemMemberRole(ctx context.Context, problemID, userID, requestUserID uuid.UUID, role problemsmodel.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID, requestUserID uuid.UUID) error
}

type SubmissionsUC interface {
	CreateSubmission(ctx context.Context, creation *submissionsmodel.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
	ListSubmissions(ctx context.Context, filter submissionsmodel.SubmissionsFilter) (*submissionsmodel.SubmissionsList, error)

	BlockSubmission(ctx context.Context, contestID, submissionID uuid.UUID, reason *string) error
	UnblockSubmission(ctx context.Context, contestID, submissionID uuid.UUID) error
	RejudgeSubmissions(ctx context.Context, filter submissionsmodel.RejudgeFilter) (int, error)
}

type TeamsUC interface {
	CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput, creatorID uuid.UUID) (*organizationsmodel.Team, error)
	GetTeam(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.Team, error)
	ListOrganizationTeams(ctx context.Context, orgID, userID uuid.UUID) ([]organizationsmodel.Team, error)
	UpdateTeam(ctx context.Context, teamID, userID uuid.UUID, input *organizationsmodel.UpdateTeamInput) error
	DeleteTeam(ctx context.Context, teamID, userID uuid.UUID) error

	// Member management
	AddTeamMember(ctx context.Context, input *organizationsmodel.AddTeamMemberInput, requestUserID uuid.UUID) error
	ListTeamMembers(ctx context.Context, teamID, requestUserID uuid.UUID) ([]organizationsmodel.TeamMember, error)
	UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole, requestUserID uuid.UUID) error
	RemoveTeamMember(ctx context.Context, teamID, userID, requestUserID uuid.UUID) error
	GetUserTeams(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.Team, error)

	// Resource associations
	GetTeamContests(ctx context.Context, teamID, requestUserID uuid.UUID) ([]contestsmodel.Contest, error)
	GetTeamProblems(ctx context.Context, teamID, requestUserID uuid.UUID) ([]problemsmodel.Problem, error)
}

type UsersUC interface {
	UpdateUser(ctx context.Context, input identitymodel.UpdateUserInput) error
	ClaimTemporaryUser(ctx context.Context, caller accessmodel.User, input identitymodel.ClaimTemporaryUserInput) (identitymodel.ClaimTemporaryUserResult, error)
	AdminSetPassword(ctx context.Context, username, newPassword string) error
	AdminSendPasswordReset(ctx context.Context, username string) error
	AdminChangeEmail(ctx context.Context, username, newEmail string, withConfirmation bool) error
	AdminResendVerification(ctx context.Context, username string) error
}

type WorkshopUC interface {
	UpdateLimits(context.Context, uuid.UUID, problems.LimitsPatch) (*problemsmodel.ProblemManifest, error)
	UpdateStatement(context.Context, uuid.UUID, string, problems.StatementPatch) (*problemsmodel.LocalizedStatement, error)
	GetStatement(context.Context, uuid.UUID, string) (*problemsmodel.LocalizedStatement, error)
	WithWorkspace(ctx context.Context, problemID uuid.UUID, fn func(context.Context) error) error
	IsInitialized(ctx context.Context, problemID uuid.UUID) (bool, error)

	UpdateProblemFile(ctx context.Context, req problemsmodel.UpdateFileRequest) error
	DeleteProblemFile(ctx context.Context, problemID uuid.UUID, path string) error
	ReadProblemFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error)
	ListProblemFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]problemsmodel.FileEntry, error)
	CompileProblemComponent(ctx context.Context, req problemsmodel.CompileComponentRequest) (*problemsmodel.CompileResult, error)
	GenerateTests(ctx context.Context, req problemsmodel.GenerateTestsRequest) error
	ValidateAllTests(ctx context.Context, problemID uuid.UUID) (*problemsmodel.ValidationReport, error)
	TestSolution(ctx context.Context, req problemsmodel.TestSolutionRequest) (*problemsmodel.TestReport, error)
	GetManifest(ctx context.Context, problemID uuid.UUID) (*problemsmodel.ProblemManifest, error)
	UpdateTestsConfig(ctx context.Context, problemID uuid.UUID, testsMeta *problemsmodel.TestsMetadata) error
	GetTestsConfig(ctx context.Context, problemID uuid.UUID) (*problemsmodel.TestsMetadata, error)
}

type ContestDetails interface {
	GetContestDetails(context.Context, string, string) (*contests.ContestDetails, error)
	GetContestProblemDetails(context.Context, string, string, uuid.UUID) (*contests.ContestProblemDetails, error)
	ReadBooklet(context.Context, string, string, string) ([]byte, error)
}
