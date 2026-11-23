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
	err = access.WithinContest(ctx, uc.transactor, input.ContestID, accessmodel.ContestManage, func(tx ContestEventsTxStore, _ access.ContestAccess) error {
		var err error
		announcement, err = tx.CreateAnnouncement(ctx, user.Id, input)
		if err != nil {
			return err
		}

		if err := tx.Notify(ctx, eventdata.Notice{Type: contestsmodel.ContestEventAnnouncementCreated, ID: announcement.ID}); err != nil {
			return fmt.Errorf("notify announcement: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return announcement, nil
}

func (uc *AnnouncementsUseCase) DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error {
	return access.WithinContest(ctx, uc.transactor, contestID, accessmodel.ContestManage, func(tx ContestEventsTxStore, _ access.ContestAccess) error {
		if err := tx.DeleteAnnouncement(ctx, id, contestID); err != nil {
			return err
		}

		return tx.Notify(ctx, eventdata.Notice{Type: contestsmodel.ContestEventAnnouncementDeleted, ID: id, ContestID: &contestID})
	})
}
