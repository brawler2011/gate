package contests

import (
	"context"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

type ContestsRepo interface {
	// History rows group by user and problem, chronologically within each group.
	GetContestScoreboardSubmissions(context.Context, uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error)
	GetUserScoreboardSubmissions(context.Context, uuid.UUID, uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error)
	GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error)
	GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error)
	GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error)
	GetContestProblem(ctx context.Context, c contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestProblem, error)
	ListContestParticipants(ctx context.Context, contestID uuid.UUID) (map[uuid.UUID]string, error)
	GetContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error)
	GetPendingContestJoinRequest(ctx context.Context, contestID, userID uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	ListContestJoinRequests(ctx context.Context, contestID uuid.UUID, status *string) ([]contestsmodel.ContestJoinRequest, error)

	GetContestTeams(context.Context, uuid.UUID) ([]contestsmodel.ContestTeam, error)
	ListContests(context.Context, contestsmodel.ListContestsFilter) (*contestsmodel.ContestsList, error)
	ListContestMembers(context.Context, contestsmodel.ParticipantsFilter) (*contestsmodel.ContestMembersList, error)
	ListVirtualRuns(context.Context, uuid.UUID) ([]contestsmodel.VirtualRun, error)
	// Rows group by user and problem, chronologically within each group.
	GetVirtualScoreboardSubmissions(context.Context, uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error)
}

type NotificationsUC interface {
	SendEmail(context.Context, notificationsmodel.Notification)
}

type OrganizationsRepo interface {
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error)
}
