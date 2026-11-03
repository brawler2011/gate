package contests

import (
	"context"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
)

type contestEventsTestStore struct {
	Contest contestsmodel.Contest
	AnnouncementsRepo
	ClarificationsRepo
	OutboxRepo
}

func (contestEventsTestStore) LockContest(context.Context, uuid.UUID) error { return nil }
func (contestEventsTestStore) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return true, nil
}
func (s contestEventsTestStore) GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error) {
	return s.Contest, nil
}

type AnnouncementsRepo interface {
	CreateAnnouncement(context.Context, *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error)
	DeleteAnnouncement(context.Context, uuid.UUID, uuid.UUID) error
}

type ClarificationsRepo interface {
	CreateClarification(context.Context, *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error)
	AnswerClarification(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (*contestsmodel.ContestClarification, error)
}

type OutboxRepo interface {
	eventdata.Writer
}
