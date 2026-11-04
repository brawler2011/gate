package contests

import (
	"context"

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
	LockContest(context.Context, uuid.UUID) error
	GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error)
	HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error)
	CreateAnnouncement(ctx context.Context, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error)
	DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error
	CreateClarification(ctx context.Context, input *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error)
	AnswerClarification(ctx context.Context, id, contestID, answeredBy uuid.UUID, answer string) (*contestsmodel.ContestClarification, error)
	Record(context.Context, eventdata.Event) error
}

type ContestEventsTransactions interface {
	Within(context.Context, func(ContestEventsTxStore) error) error
}

type ScoreboardStore interface {
	GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error)
	GetContestMember(context.Context, *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error)
	GetSubmissionsForScoreboard(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) ([]contestsmodel.SubmissionForScoreboard, error)
	UpsertContestProblemResult(context.Context, *contestsmodel.UpsertContestProblemResultParams) error
}

type CommandTxStore interface {
	GetContestJoinRequestByID(context.Context, uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	CreateNotification(context.Context, *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error)
	GetPendingContestJoinRequest(context.Context, uuid.UUID, uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
	CreateContestJoinRequest(context.Context, *contestsmodel.CreateContestJoinRequestInput) (*contestsmodel.ContestJoinRequest, error)

	UpdateContest(context.Context, contestsmodel.ContestUpdateInput) error
	DeleteContest(context.Context, uuid.UUID) error
	CreateContestProblem(context.Context, contestsmodel.ContestProblemCreation) error
	UpdateContestProblemPackage(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	DeleteContestProblem(context.Context, contestsmodel.ContestProblemDeletion) error
	ReorderContestProblems(context.Context, uuid.UUID, []contestsmodel.ContestProblemReorderItem) error
	UpdateContestMember(context.Context, uuid.UUID, uuid.UUID, string) error
	DeleteContestMember(context.Context, uuid.UUID, uuid.UUID) error
	GetMember(context.Context, uuid.UUID, uuid.UUID) (*organizationsmodel.OrganizationMember, error)
	CreateContestTeam(context.Context, uuid.UUID, uuid.UUID, contestsmodel.ContestRole) error
	UpdateContestTeamRole(context.Context, uuid.UUID, uuid.UUID, contestsmodel.ContestRole) error
	DeleteContestTeam(context.Context, uuid.UUID, uuid.UUID) error

	HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error)
	HasOrganizationPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.OrgAction) (bool, error)
	ScoreboardStore
	eventdata.Writer
	LockContest(context.Context, uuid.UUID) error
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
