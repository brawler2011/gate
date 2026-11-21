package contests

import (
	"context"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type AnnouncementsUseCase struct {
	transactor ContestEventsTransactions
	reads      AnnouncementReads
	access     access.Loader
}

func NewAnnouncementsUseCase(transactor ContestEventsTransactions, reads AnnouncementReads, loader access.Loader) *AnnouncementsUseCase {
	return &AnnouncementsUseCase{transactor: transactor, reads: reads, access: loader}
}

func (uc *AnnouncementsUseCase) CreateAnnouncement(ctx context.Context, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error) {
	if input.Title == "" {
		return nil, pkg.AtField("/title", pkg.Invalid(pkg.ErrRequired, "title cannot be empty", nil))
	}
	if input.Body == "" {
		return nil, pkg.AtField("/body", pkg.Invalid(pkg.ErrRequired, "body cannot be empty", nil))
	}

	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	var announcement *contestsmodel.ContestAnnouncement
	err = withinContestEvents(ctx, uc.transactor, input.ContestID, accessmodel.ContestManage, func(tx ContestEventsTxStore, _ access.ContestAccess) error {
		var err error
		announcement, err = tx.CreateAnnouncement(ctx, user.Id, input)
		if err != nil {
			return err
		}

		event := contestsmodel.ContestAnnouncementCreatedEvent{
			ID:             announcement.ID,
			ContestID:      announcement.ContestID,
			ProblemID:      announcement.ProblemID,
			ProblemTitle:   announcement.ProblemTitle,
			ProblemLetter:  announcement.ProblemLetter,
			AuthorID:       announcement.AuthorID,
			AuthorUsername: announcement.AuthorUsername,
			Title:          announcement.Title,
			Body:           announcement.Body,
			CreatedAt:      announcement.CreatedAt,
		}

		outboxEvent := eventdata.Event{
			AggregateID: announcement.ContestID,
			Type:        eventdata.OutboxEventContestAnnouncementCreated,
			Payload:     event,
		}

		if err := tx.Record(ctx, outboxEvent); err != nil {
			return fmt.Errorf("create outbox event for announcement: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return announcement, nil
}

func (uc *AnnouncementsUseCase) DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error {
	return withinContestEvents(ctx, uc.transactor, contestID, accessmodel.ContestManage, func(tx ContestEventsTxStore, _ access.ContestAccess) error {
		if err := tx.DeleteAnnouncement(ctx, id, contestID); err != nil {
			return err
		}

		event := contestsmodel.ContestAnnouncementDeletedEvent{
			ID:        id,
			ContestID: contestID,
		}

		outboxEvent := eventdata.Event{
			AggregateID: contestID,
			Type:        eventdata.OutboxEventContestAnnouncementDeleted,
			Payload:     event,
		}

		return tx.Record(ctx, outboxEvent)
	})
}
