package contests

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

type AnnouncementsRepo interface {
	CreateAnnouncement(context.Context, *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error)
	DeleteAnnouncement(context.Context, uuid.UUID, uuid.UUID) error
}

type ClarificationsRepo interface {
	CreateClarification(context.Context, *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error)
	AnswerClarification(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (*contestsmodel.ContestClarification, error)
}

type ContestsRepo interface {
	ScoreboardStore
	CreateContest(ctx context.Context, params *contestsmodel.CreateContestParams) error
	GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error)

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

	CreateContestTeam(ctx context.Context, contestID, teamID uuid.UUID, role contestsmodel.ContestRole) error
	UpdateContestTeamRole(ctx context.Context, contestID, teamID uuid.UUID, role contestsmodel.ContestRole) error
	DeleteContestTeam(ctx context.Context, contestID, teamID uuid.UUID) error

	GetContestScoreboardFromStandings(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ContestProblemResult, map[uuid.UUID]string, error)

	CreateContestUserProblemBlock(ctx context.Context, params *contestsmodel.CreateContestUserProblemBlockParams) error
	DeleteContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) error
	GetContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error)

	// Contest Join Requests
	CreateContestJoinRequest(ctx context.Context, input *contestsmodel.CreateContestJoinRequestInput) (*contestsmodel.ContestJoinRequest, error)
	GetContestJoinRequestByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	GetPendingContestJoinRequest(ctx context.Context, contestID, userID uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	ListContestJoinRequests(ctx context.Context, contestID uuid.UUID, status *string) ([]contestsmodel.ContestJoinRequest, error)

	UpdateContestJoinRequestStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, reviewedBy *uuid.UUID) error
}

type ContestsUC interface {
	GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error)
}

type DraftsRepo interface {
	CreateDraft(ctx context.Context, creation *contestsmodel.ContestDraftCreation) (uuid.UUID, error)
	GetDraft(ctx context.Context, id uuid.UUID) (contestsmodel.ContestDraft, error)
	GetDraftsCount(ctx context.Context, contestID, userID uuid.UUID) (int64, error)
	DeleteDraft(ctx context.Context, id uuid.UUID) error
}

type NotificationsUC interface {
	Notify(ctx context.Context, input *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error)
}

type OrganizationsRepo interface {

	// Member management

	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)

	// Invitations
}

type OutboxRepo interface {
	eventdata.Writer
}

type PermissionsUC interface {
}

type SubmissionsRepo interface {
	BlockUserProblemSubmissions(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string) error
}

type UsersRepo interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
}
