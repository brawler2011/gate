package contests

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
)

type ContestEventsTxStore interface {
	access.Loader
	CreateAnnouncement(ctx context.Context, authorID uuid.UUID, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error)
	DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error
	CreateClarification(ctx context.Context, userID uuid.UUID, input *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error)
	AnswerClarification(ctx context.Context, id, contestID, answeredBy uuid.UUID, answer string) (*contestsmodel.ContestClarification, error)
	eventdata.Writer
}

type ContestEventsTransactions interface {
	Within(context.Context, func(ContestEventsTxStore) error) error
}

type CommandTxStore interface {
	LockContestProblemOrder(context.Context, uuid.UUID) error
	LockSubmissionTuple(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	GetContestJoinRequestByID(context.Context, uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	CreateNotification(context.Context, *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error)
	GetPendingContestJoinRequest(context.Context, uuid.UUID, uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	ListContestOwners(context.Context, uuid.UUID) ([]uuid.UUID, error)
	CreateContestJoinRequest(context.Context, *contestsmodel.CreateContestJoinRequestInput) (*contestsmodel.ContestJoinRequest, error)

	UpdateContest(context.Context, contestsmodel.ContestUpdateInput) error
	DeleteContest(context.Context, uuid.UUID) error
	CreateContestProblem(context.Context, contestsmodel.ContestProblemCreation) error
	UpdateContestProblemPackage(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	DeleteContestProblem(context.Context, contestsmodel.ContestProblemDeletion) error
	ReorderContestProblems(context.Context, uuid.UUID, []contestsmodel.ContestProblemReorderItem) error
	UpdateContestMember(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestRole) error
	GetContestTeam(context.Context, uuid.UUID, uuid.UUID) (contestsmodel.ContestTeam, error)
	ResolveContestProblemPackage(context.Context, uuid.UUID, uuid.UUID, bool) (contestsmodel.PackageRef, error)
	DeleteContestMember(context.Context, uuid.UUID, uuid.UUID) error
	GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	CreateContestTeam(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestRole) error
	UpdateContestTeamRole(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestRole) error
	DeleteContestTeam(context.Context, uuid.UUID, uuid.UUID) error
	CreateVirtualRun(context.Context, uuid.UUID, contestsmodel.VirtualRun) error
	FinishVirtualRun(context.Context, uuid.UUID, uuid.UUID, time.Time) error

	access.Loader
	GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error)
	GetContestMember(context.Context, *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error)
	eventdata.Writer
	CreateContest(context.Context, *contestsmodel.CreateContestParams) error
	CreateContestMember(context.Context, *contestsmodel.CreateContestMemberParams) error
	UpdateContestJoinRequestStatus(context.Context, uuid.UUID, organizationsmodel.RequestStatus, *uuid.UUID) error
	CreateContestUserProblemBlock(context.Context, *contestsmodel.CreateContestUserProblemBlockParams) error
	DeleteContestUserProblemBlock(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	BlockUserProblemSubmissions(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *string) ([]submissionsmodel.SubmissionReset, error)
	ResetSubmissionsState(context.Context, submissionsmodel.RejudgeFilter) ([]submissionsmodel.SubmissionReset, error)
	GetSubmission(context.Context, uuid.UUID) (submissionsmodel.Submission, error)
	ScheduleJob(context.Context, uuid.UUID) (judgingmodel.Job, error)
}

type CommandTransactions interface {
	Within(context.Context, func(CommandTxStore) error) error
}
