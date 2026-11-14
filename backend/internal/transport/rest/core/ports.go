package core

import (
	"context"
	"io"

	paging "github.com/brawler2011/gate/backend/internal/page"

	"github.com/brawler2011/gate/backend/internal/blogs"
	"github.com/brawler2011/gate/backend/internal/identity"

	contests "github.com/brawler2011/gate/backend/internal/contests"
	"github.com/brawler2011/gate/backend/internal/problems"

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
	GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error)
	CreateContest(ctx context.Context, c *contestsmodel.CreateContestInput) (uuid.UUID, error)
	UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	CreateParticipant(ctx context.Context, c contestsmodel.ParticipantCreation) error

	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
	DeleteParticipant(ctx context.Context, c contestsmodel.ParticipantDeletion) error

	CreateContestTeam(ctx context.Context, contestID, teamID uuid.UUID, role contestsmodel.ContestRole) error
	UpdateContestTeamRole(ctx context.Context, contestID, teamID uuid.UUID, role contestsmodel.ContestRole) error
	DeleteContestTeam(ctx context.Context, contestID, teamID uuid.UUID) error

	CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error

	DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error
	ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error

	GetContestScoreboard(ctx context.Context, contestID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error)

	BlockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string) error
	UnblockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, rejudgeSubmissions bool) error
	GetProblemBlockStatusForUser(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error)

	// Contest Join Requests
	CreateJoinRequest(ctx context.Context, orgLogin, contestLogin string, message *string) (*contestsmodel.ContestJoinRequest, bool, error)
	ListJoinRequests(ctx context.Context, orgLogin, contestLogin string) ([]contestsmodel.ContestJoinRequest, error)
	CancelJoinRequest(ctx context.Context, orgLogin, contestLogin string) error
	ApproveJoinRequest(ctx context.Context, orgLogin, contestLogin string, requestID uuid.UUID) error
	RejectJoinRequest(ctx context.Context, orgLogin, contestLogin string, requestID uuid.UUID) error
	GetMyPendingJoinRequest(ctx context.Context, orgLogin, contestLogin string) (*contestsmodel.ContestJoinRequest, error)

	GetContestTeams(context.Context, uuid.UUID) ([]contestsmodel.ContestTeam, error)
	ListPublicContests(context.Context, contestsmodel.PublicContestsFilter) (*contestsmodel.ContestsList, error)
	ListWorkshopContests(context.Context, contestsmodel.WorkshopContestsFilter) (*contestsmodel.ContestsList, error)
	ListUserContests(context.Context, contestsmodel.UserContestsFilter) (*contestsmodel.ContestsList, error)
	ListAdminContests(context.Context, contestsmodel.AdminContestsFilter) (*contestsmodel.ContestsList, error)
	ListOrganizationContests(context.Context, string, string, int32, int32) (*contestsmodel.ContestsList, error)
	ListContestMembers(context.Context, contestsmodel.ParticipantsFilter) (*contestsmodel.ContestMembersList, error)
	ListDashboardContests(context.Context, uuid.UUID, int32) ([]contestsmodel.DashboardContest, error)
}

type DraftsUC interface {
	CreateDraft(ctx context.Context, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error)
	DeleteDraft(ctx context.Context, contestID, draftID uuid.UUID) error
	ListDrafts(ctx context.Context, filter contestsmodel.ContestDraftsFilter) (*contestsmodel.ContestDraftsList, error)
}

type NotificationsUC interface {
	MarkAsRead(ctx context.Context, id uuid.UUID) error
	MarkAllAsRead(ctx context.Context) error
}

type OrganizationsUC interface {
	CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error)
	GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error)
	ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter) (*organizationsmodel.OrganizationList, error)
	UpdateOrganizationByLogin(ctx context.Context, login string, input *organizationsmodel.UpdateOrganizationInput) error
	DeleteOrganizationByLogin(ctx context.Context, login string) error

	// Member management
	AddMemberByLogin(ctx context.Context, login string, input *organizationsmodel.AddOrganizationMemberInput) error
	ListMembersByLogin(ctx context.Context, login string, request paging.Request) (*paging.List[organizationsmodel.OrganizationMember], error)
	RemoveMemberByLogin(ctx context.Context, login string, userID uuid.UUID) error
	BatchCreateUsers(ctx context.Context, input organizationsmodel.BatchCreateOrganizationUsersInput) (*organizationsmodel.BatchCreateOrganizationUsersResult, error)

	// Invitations
	InviteMember(ctx context.Context, orgLogin string, targetUserID uuid.UUID, role organizationsmodel.OrganizationRole) (*organizationsmodel.OrganizationInvitation, error)
	ListInvitations(ctx context.Context, orgLogin string) ([]organizationsmodel.OrganizationInvitation, error)
	CancelInvitation(ctx context.Context, orgLogin string, invitationID uuid.UUID) error
	AcceptInvitation(ctx context.Context, invitationID uuid.UUID) error
	DeclineInvitation(ctx context.Context, invitationID uuid.UUID) error

	// Join Requests
	CreateJoinRequest(ctx context.Context, orgLogin string, message *string) (*organizationsmodel.OrganizationJoinRequest, bool, error)
	ListJoinRequests(ctx context.Context, orgLogin string) ([]organizationsmodel.OrganizationJoinRequest, error)
	CancelJoinRequest(ctx context.Context, orgLogin string) error
	ApproveJoinRequest(ctx context.Context, orgLogin string, requestID uuid.UUID, role organizationsmodel.OrganizationRole) error
	RejectJoinRequest(ctx context.Context, orgLogin string, requestID uuid.UUID) error
	GetMyPendingJoinRequest(ctx context.Context, orgLogin string) (*organizationsmodel.OrganizationJoinRequest, error)
}

type PermissionsUC interface {
	GetContestPermissions(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*accessmodel.ContestPermissions, error)

	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action accessmodel.ContestAction) (bool, error)

	GetEffectiveContestRole(ctx context.Context, contestID uuid.UUID, userID uuid.UUID) (*contestsmodel.ContestRole, error)
}

type ProblemsUC interface {
	GetProblemDetails(context.Context, uuid.UUID) (*problemsmodel.ProblemDetails, error)
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error

	AddProblemTeam(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID uuid.UUID) error

	CreateProblemMember(ctx context.Context, problemID, userID uuid.UUID, role problemsmodel.ProblemRole) error
	UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role problemsmodel.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error

	ListProblemMembers(context.Context, uuid.UUID, paging.Request) (*paging.List[problemsmodel.ProblemMember], error)
	GetProblemTeams(context.Context, uuid.UUID) ([]problemsmodel.ProblemTeam, error)
	ListProblems(context.Context, *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error)
	ListDashboardProblems(context.Context, uuid.UUID, int32) ([]problemsmodel.DashboardProblem, error)
	ListTemplates(context.Context, uuid.UUID) ([]problems.Template, error)
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
	CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error)
	GetTeam(ctx context.Context, teamID uuid.UUID) (*organizationsmodel.Team, error)
	ListTeams(ctx context.Context, filter *organizationsmodel.TeamsFilter) (*organizationsmodel.TeamsList, error)
	UpdateTeam(ctx context.Context, teamID uuid.UUID, input *organizationsmodel.UpdateTeamInput) error
	DeleteTeam(ctx context.Context, teamID uuid.UUID) error

	// Member management
	AddTeamMember(ctx context.Context, input *organizationsmodel.AddTeamMemberInput) error
	ListTeamMembers(ctx context.Context, teamID uuid.UUID, request paging.Request) (*paging.List[organizationsmodel.TeamMember], error)
	UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error
	RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error

	// Resource associations
	GetTeamContests(ctx context.Context, teamID uuid.UUID, request paging.Request) (*paging.List[contestsmodel.Contest], error)
	GetTeamProblems(ctx context.Context, teamID uuid.UUID, request paging.Request) (*paging.List[problemsmodel.Problem], error)
}

type UsersUC interface {
	UpdateUser(ctx context.Context, input identitymodel.UpdateUserInput) error
	ClaimTemporaryUser(ctx context.Context, caller accessmodel.User, input identitymodel.ClaimTemporaryUserInput) (identitymodel.ClaimTemporaryUserResult, error)
	AdminSetPassword(ctx context.Context, username, newPassword string) error
	AdminSendPasswordReset(ctx context.Context, username string) error
	AdminChangeEmail(ctx context.Context, username, newEmail string, withConfirmation bool) error
	AdminResendVerification(ctx context.Context, username string) error
}

type ContestDetails interface {
	GetContestDetails(context.Context, string, string) (*contests.ContestDetails, error)
	GetContestProblemDetails(context.Context, string, string, uuid.UUID) (*contests.ContestProblemDetails, error)
	ReadBooklet(context.Context, string, string, string) ([]byte, error)
	OpenProblemAsset(context.Context, string, string, uuid.UUID, string) (io.ReadCloser, string, error)
}

type AvatarsUC interface {
	UploadAvatar(ctx context.Context, username string, fileReader io.Reader, filename, contentType string) (uuid.UUID, error)
	DeleteAvatar(ctx context.Context, username string) error
	GetAvatar(ctx context.Context, username string, ifNoneMatch *string) (identity.AvatarImage, error)
}

type BlogsUC interface {
	CreatePost(ctx context.Context, title, text, description string, imageReader io.Reader, filename string) (uuid.UUID, error)
	UpdatePost(ctx context.Context, id uuid.UUID, title, text, description *string, imageReader io.Reader, filename string) error
	DeletePost(ctx context.Context, id uuid.UUID) error
	GetPostImage(ctx context.Context, imageKey string, ifNoneMatch *string) (blogs.PostImage, error)
}

type AnnouncementsUC interface {
	CreateAnnouncement(ctx context.Context, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error)
	DeleteAnnouncement(ctx context.Context, announcementID, contestID uuid.UUID) error
	ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*contestsmodel.ContestAnnouncementsList, error)
}

type ClarificationsUC interface {
	CreateClarification(ctx context.Context, input *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error)
	AnswerClarification(ctx context.Context, input *contestsmodel.AnswerContestClarificationInput) (*contestsmodel.ContestClarification, error)
	ListClarifications(context.Context, uuid.UUID, *contestsmodel.ContestClarificationsFilter) (*contestsmodel.ContestClarificationsList, error)
}
