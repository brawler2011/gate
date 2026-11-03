package contests

import (
	"context"

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
