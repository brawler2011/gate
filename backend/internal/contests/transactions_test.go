package contests

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
)

type contestEventsTestStore struct {
	*accesstest.Loader
	AnnouncementsRepo
	ClarificationsRepo
	OutboxRepo
}

type AnnouncementsRepo interface {
	CreateAnnouncement(context.Context, uuid.UUID, *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error)
	DeleteAnnouncement(context.Context, uuid.UUID, uuid.UUID) error
}

type ClarificationsRepo interface {
	CreateClarification(context.Context, uuid.UUID, *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error)
	AnswerClarification(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (*contestsmodel.ContestClarification, error)
}

type OutboxRepo interface {
	eventdata.Writer
}
