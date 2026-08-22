package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type AnnouncementsUseCase struct {
	announcementsRepo interfaces.AnnouncementsRepo
	contestsUC        interfaces.ContestsUC
	outboxRepo        interfaces.OutboxRepo
	transactor        interfaces.Transactor
}

func NewAnnouncementsUseCase(
	announcementsRepo interfaces.AnnouncementsRepo,
	contestsUC interfaces.ContestsUC,
	outboxRepo interfaces.OutboxRepo,
	transactor interfaces.Transactor,
) *AnnouncementsUseCase {
	return &AnnouncementsUseCase{
		announcementsRepo: announcementsRepo,
		contestsUC:        contestsUC,
		outboxRepo:        outboxRepo,
		transactor:        transactor,
	}
}

func (uc *AnnouncementsUseCase) CreateAnnouncement(ctx context.Context, input *models.CreateContestAnnouncementInput) (*models.ContestAnnouncement, error) {
	if input.Title == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "title cannot be empty")
	}
	if input.Body == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "body cannot be empty")
	}

	var announcement *models.ContestAnnouncement
	err := uc.transactor.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		announcement, err = uc.announcementsRepo.WithTx(tx).CreateAnnouncement(ctx, input)
		if err != nil {
			return err
		}

		event := models.ContestAnnouncementCreatedEvent{
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

		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal announcement created event: %w", err)
		}

		headers := make(map[string]string)
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))

		outboxEvent := &models.CreateOutboxEventParams{
			Id:          uuid.New(),
			AggregateID: announcement.ContestID,
			EventType:   models.OutboxEventContestAnnouncementCreated,
			Payload:     payload,
			Headers:     headers,
		}

		if err := uc.outboxRepo.WithTx(tx).CreateEvent(ctx, outboxEvent); err != nil {
			return fmt.Errorf("create outbox event for announcement: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return announcement, nil
}

func (uc *AnnouncementsUseCase) ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*models.ContestAnnouncementsList, error) {
	return uc.announcementsRepo.ListAnnouncements(ctx, contestID, page, pageSize)
}

func (uc *AnnouncementsUseCase) DeleteAnnouncement(ctx context.Context, id, contestID uuid.UUID) error {
	return uc.transactor.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := uc.announcementsRepo.WithTx(tx).DeleteAnnouncement(ctx, id, contestID); err != nil {
			return err
		}

		event := models.ContestAnnouncementDeletedEvent{
			ID:        id,
			ContestID: contestID,
		}

		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal announcement deleted event: %w", err)
		}

		headers := make(map[string]string)
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))

		outboxEvent := &models.CreateOutboxEventParams{
			Id:          uuid.New(),
			AggregateID: contestID,
			EventType:   models.OutboxEventContestAnnouncementDeleted,
			Payload:     payload,
			Headers:     headers,
		}

		return uc.outboxRepo.WithTx(tx).CreateEvent(ctx, outboxEvent)
	})
}
