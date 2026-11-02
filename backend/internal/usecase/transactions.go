package usecase

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

type TeamTxStore interface {
	CreateTeam(ctx context.Context, input *models.CreateTeamInput) (*models.Team, error)
	AddTeamMember(ctx context.Context, teamID, userID uuid.UUID, role models.TeamRole) error
}
type TeamTransactions interface {
	Within(context.Context, func(TeamTxStore) error) error
}

type ProblemTxStore interface {
	CreateProblem(ctx context.Context, params *models.CreateProblemParams) error
	CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error
}
type ProblemTransactions interface {
	Within(context.Context, func(ProblemTxStore) error) error
}

type IdentityTxStore interface {
	CreateUser(ctx context.Context, params models.CreateUserParams) error
	GetUserById(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	GetUserByUsernameOrEmail(ctx context.Context, identifier string) (models.User, error)
	UpdateUser(ctx context.Context, params models.UpdateUserParams) error
	SetUserEmailVerified(ctx context.Context, id uuid.UUID, isVerified bool) error
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateUserEmail(ctx context.Context, id uuid.UUID, email string, isVerified bool) error
	ClaimTemporaryUser(ctx context.Context, id, claimedByUserID uuid.UUID, claimedAt time.Time) error
	ListUserContestMemberships(ctx context.Context, userID uuid.UUID) ([]models.UserContestMembership, error)
	AddContestMemberIfNotExists(ctx context.Context, contestID, userID uuid.UUID, role string) error
	CreateSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (models.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	DeleteSessionsByUserID(ctx context.Context, userID uuid.UUID) error
	UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
	CleanupExpiredSessions(ctx context.Context, hardLimitCutoff time.Time) error
	CreateAuthToken(ctx context.Context, params models.CreateAuthTokenParams) error
	GetAuthTokenByHash(ctx context.Context, tokenHash string, tokenType models.AuthTokenType) (models.AuthToken, error)
	DeleteAuthToken(ctx context.Context, id uuid.UUID) error
	DeleteAuthTokensByUserIdAndType(ctx context.Context, userID uuid.UUID, tokenType models.AuthTokenType) error
	CleanupExpiredAuthTokens(ctx context.Context) error
}
type IdentityTransactions interface {
	Within(context.Context, func(IdentityTxStore) error) error
}

type OrganizationTxStore interface {
	CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error)
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role models.OrganizationRole) error
	CreateUser(ctx context.Context, params models.CreateUserParams) error
	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status models.RequestStatus) error
	UpdateJoinRequestStatus(ctx context.Context, id uuid.UUID, status models.RequestStatus, reviewedBy *uuid.UUID) error
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*models.OrganizationInvitation, error)
	GetJoinRequestByID(ctx context.Context, id uuid.UUID) (*models.OrganizationJoinRequest, error)
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrganizationMember, error)
}
type OrganizationTransactions interface {
	Within(context.Context, func(OrganizationTxStore) error) error
}

type SubmissionTxStore interface {
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error)
	CreateEvent(ctx context.Context, event *models.CreateOutboxEventParams) error
	ResetSubmissionsState(ctx context.Context, filter models.RejudgeFilter) ([]models.SubmissionReset, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) (models.SubmissionChange, error)
	BlockSubmission(ctx context.Context, id uuid.UUID, reason *string) error
}
type SubmissionTransactions interface {
	Within(context.Context, func(SubmissionTxStore) error) error
}

type ContestEventsTxStore interface {
	CreateAnnouncement(ctx context.Context, input *models.CreateContestAnnouncementInput) (*models.ContestAnnouncement, error)
	DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error
	CreateClarification(ctx context.Context, input *models.CreateContestClarificationInput) (*models.ContestClarification, error)
	AnswerClarification(ctx context.Context, id, contestID, answeredBy uuid.UUID, answer string) (*models.ContestClarification, error)
	CreateEvent(ctx context.Context, event *models.CreateOutboxEventParams) error
}
type ContestEventsTransactions interface {
	Within(context.Context, func(ContestEventsTxStore) error) error
}
