package contests

import (
	"context"
	"fmt"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type AnnouncementsUseCase struct {
	transactor  ContestEventsTransactions
	reads       AnnouncementReads
	permissions Permissions
}

func NewAnnouncementsUseCase(transactor ContestEventsTransactions, reads AnnouncementReads, permissions Permissions) *AnnouncementsUseCase {
	return &AnnouncementsUseCase{transactor: transactor, reads: reads, permissions: permissions}
}

func (uc *AnnouncementsUseCase) CreateAnnouncement(ctx context.Context, input *contestsmodel.CreateContestAnnouncementInput) (*contestsmodel.ContestAnnouncement, error) {
	if input.Title == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "title cannot be empty")
	}
	if input.Body == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "body cannot be empty")
	}

	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}
	if input.AuthorID != user.Id {
		return nil, pkg.NoPermission
	}
	var announcement *contestsmodel.ContestAnnouncement
	err := withinContestEvents(ctx, uc.transactor, input.ContestID, accessmodel.ActionManageContest, func(tx ContestEventsTxStore) error {
		var err error
		announcement, err = tx.CreateAnnouncement(ctx, input)
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
	return withinContestEvents(ctx, uc.transactor, contestID, accessmodel.ActionManageContest, func(tx ContestEventsTxStore) error {
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
