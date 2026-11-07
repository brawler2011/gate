package identity

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

type OrganizationsRepo interface {
	CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error)
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Organization, error)
	GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error)
	ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter) ([]organizationsmodel.Organization, int32, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, input *organizationsmodel.UpdateOrganizationInput) error
	DeleteOrganization(ctx context.Context, id uuid.UUID) error

	// Member management
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]organizationsmodel.OrganizationMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.Organization, error)
	ResolveUserOrganizationID(ctx context.Context, userID uuid.UUID, requestedOrgID *uuid.UUID) (uuid.UUID, bool, error)

	// Invitations
	CreateInvitation(ctx context.Context, input *organizationsmodel.CreateOrganizationInvitationInput) (*organizationsmodel.OrganizationInvitation, error)
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	GetPendingInvitation(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationInvitation, error)
	ListInvitations(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationInvitation, error)
	ListUserInvitations(ctx context.Context, userID uuid.UUID, status *string) ([]organizationsmodel.OrganizationInvitation, error)
	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus) error

	// Join Requests
	CreateJoinRequest(ctx context.Context, input *organizationsmodel.CreateOrganizationJoinRequestInput) (*organizationsmodel.OrganizationJoinRequest, error)
	GetJoinRequestByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
	GetPendingJoinRequest(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error)
	ListJoinRequests(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationJoinRequest, error)
	ListUserJoinRequests(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.OrganizationJoinRequest, error)
	UpdateJoinRequestStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, reviewedBy *uuid.UUID) error
}

type ProblemsRepo interface {
	CreateProblem(ctx context.Context, params *problemsmodel.CreateProblemParams) error
	CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error)
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error)
	GetProblemTeams(ctx context.Context, problemId uuid.UUID) ([]problemsmodel.ProblemTeam, error)
	AddProblemTeam(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error
	UpdateProblemTeamPermission(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error
	RemoveProblemTeam(ctx context.Context, problemID, teamID uuid.UUID) error
	UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role problemsmodel.ProblemRole) error
	RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error
	WorkspaceRepo
}

type TeamsRepo interface {
	CreateTeam(ctx context.Context, input *organizationsmodel.CreateTeamInput) (*organizationsmodel.Team, error)
	GetTeamByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Team, error)
	GetTeamBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*organizationsmodel.Team, error)
	ListTeams(ctx context.Context, filter *organizationsmodel.TeamsFilter, userID uuid.UUID, canManage bool) ([]organizationsmodel.Team, int32, error)
	UpdateTeam(ctx context.Context, id uuid.UUID, input *organizationsmodel.UpdateTeamInput) error
	DeleteTeam(ctx context.Context, id uuid.UUID) error

	// Member management
	AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error
	GetTeamMember(ctx context.Context, teamID, userID uuid.UUID) (*organizationsmodel.TeamMember, error)
	ListTeamMembers(ctx context.Context, teamID uuid.UUID) ([]organizationsmodel.TeamMember, error)
	UpdateTeamMemberRole(ctx context.Context, teamID, userID uuid.UUID, role organizationsmodel.TeamRole) error
	RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error

	// User's teams
	GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]organizationsmodel.Team, error)

	// Resource associations
	GetTeamContests(ctx context.Context, teamID, viewerID uuid.UUID) ([]contestsmodel.Contest, error)
	GetTeamProblems(ctx context.Context, teamID, viewerID uuid.UUID) ([]problemsmodel.Problem, error)
}

type UsersUC interface {
	CreateUser(ctx context.Context, input identitymodel.CreateUserInput) (uuid.UUID, error)
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
	GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error)
	UpdateUser(ctx context.Context, input identitymodel.UpdateUserInput) error
	ClaimTemporaryUser(ctx context.Context, caller accessmodel.User, input identitymodel.ClaimTemporaryUserInput) (identitymodel.ClaimTemporaryUserResult, error)
	AdminSetPassword(ctx context.Context, username, newPassword string) error
	AdminSendPasswordReset(ctx context.Context, username string) error
	AdminChangeEmail(ctx context.Context, username, newEmail string, withConfirmation bool) error
	AdminResendVerification(ctx context.Context, username string) error
}

// WorkspaceRepo stores only the location of the canonical GFMT package and its
// searchable catalog fields. CommitWorkspace atomically checks the old hash,
// switches the package reference and updates the catalog.
type WorkspaceRepo interface {
	GetWorkspaceHash(ctx context.Context, id uuid.UUID) (string, error)
	CommitWorkspace(ctx context.Context, id uuid.UUID, previousHash, hash string, metadata *problemsmodel.WorkspaceMetadata) error
}
