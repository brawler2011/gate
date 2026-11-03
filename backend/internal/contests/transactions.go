package contests

import (
	"context"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
)

type ContestEventsTxStore interface {
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
