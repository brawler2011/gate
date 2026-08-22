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

type ClarificationsUseCase struct {
	clarificationsRepo interfaces.ClarificationsRepo
	announcementsRepo   interfaces.AnnouncementsRepo
	contestsUC         interfaces.ContestsUC
	outboxRepo         interfaces.OutboxRepo
	transactor         interfaces.Transactor
}

func NewClarificationsUseCase(
	clarificationsRepo interfaces.ClarificationsRepo,
	announcementsRepo interfaces.AnnouncementsRepo,
	contestsUC interfaces.ContestsUC,
	outboxRepo interfaces.OutboxRepo,
	transactor interfaces.Transactor,
) *ClarificationsUseCase {
	return &ClarificationsUseCase{
		clarificationsRepo: clarificationsRepo,
		announcementsRepo:   announcementsRepo,
		contestsUC:         contestsUC,
		outboxRepo:         outboxRepo,
		transactor:         transactor,
	}
}

func (uc *ClarificationsUseCase) CreateClarification(ctx context.Context, input *models.CreateContestClarificationInput) (*models.ContestClarification, error) {
	if input.Question == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "question cannot be empty")
	}

	contest, err := uc.contestsUC.GetContest(ctx, input.ContestID)
	if err != nil {
		return nil, err
	}

	// Check if clarifications are allowed
	settings := models.MapToContestSettings(contest.Settings)
	if settings.AllowClarifications != nil && !*settings.AllowClarifications {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "clarifications are disabled for this contest")
	}

	var clarification *models.ContestClarification
	err = uc.transactor.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		clarification, err = uc.clarificationsRepo.WithTx(tx).CreateClarification(ctx, input)
		if err != nil {
			return err
		}

		event := models.ContestClarificationCreatedEvent{
			ID:            clarification.ID,
			ContestID:     clarification.ContestID,
			ProblemID:     clarification.ProblemID,
			ProblemTitle:  clarification.ProblemTitle,
			ProblemLetter: clarification.ProblemLetter,
			UserID:        clarification.UserID,
			Username:      clarification.Username,
			Question:      clarification.Question,
			CreatedAt:     clarification.CreatedAt,
		}

		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal clarification created event: %w", err)
		}

		headers := make(map[string]string)
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))

		outboxEvent := &models.CreateOutboxEventParams{
			Id:          uuid.New(),
			AggregateID: clarification.ContestID,
			EventType:   models.OutboxEventContestClarificationCreated,
			Payload:     payload,
			Headers:     headers,
		}

		return uc.outboxRepo.WithTx(tx).CreateEvent(ctx, outboxEvent)
	})
	if err != nil {
		return nil, err
	}

	return clarification, nil
}

func (uc *ClarificationsUseCase) ListClarificationsForUser(ctx context.Context, contestID, userID uuid.UUID, page, pageSize int32) (*models.ContestClarificationsList, error) {
	return uc.clarificationsRepo.ListClarificationsForUser(ctx, contestID, userID, page, pageSize)
}

func (uc *ClarificationsUseCase) ListClarificationsForModerator(ctx context.Context, contestID uuid.UUID, filter *models.ContestClarificationsFilter) (*models.ContestClarificationsList, error) {
	return uc.clarificationsRepo.ListClarificationsForModerator(ctx, contestID, filter)
}

func (uc *ClarificationsUseCase) AnswerClarification(ctx context.Context, input *models.AnswerContestClarificationInput) (*models.ContestClarification, error) {
	if input.Answer == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "answer cannot be empty")
	}

	var clarification *models.ContestClarification
	err := uc.transactor.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		clarification, err = uc.clarificationsRepo.WithTx(tx).AnswerClarification(
			ctx,
			input.ClarificationID,
			input.ContestID,
			input.AnsweredBy,
			input.Answer,
		)
		if err != nil {
			return err
		}

		ansByUsername := ""
		if clarification.AnsweredByUsername != nil {
			ansByUsername = *clarification.AnsweredByUsername
		}
		ansAt := clarification.CreatedAt
		if clarification.AnsweredAt != nil {
			ansAt = *clarification.AnsweredAt
		}

		event := models.ContestClarificationAnsweredEvent{
			ID:                 clarification.ID,
			ContestID:          clarification.ContestID,
			ProblemID:          clarification.ProblemID,
			ProblemTitle:       clarification.ProblemTitle,
			ProblemLetter:      clarification.ProblemLetter,
			UserID:             clarification.UserID,
			Username:           clarification.Username,
			Question:           clarification.Question,
			Answer:             input.Answer,
			AnsweredBy:         input.AnsweredBy,
			AnsweredByUsername: ansByUsername,
			AnsweredAt:         ansAt,
		}

		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal clarification answered event: %w", err)
		}

		headers := make(map[string]string)
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))

		outboxEvent := &models.CreateOutboxEventParams{
			Id:          uuid.New(),
			AggregateID: clarification.ContestID,
			EventType:   models.OutboxEventContestClarificationAnswered,
			Payload:     payload,
			Headers:     headers,
		}

		if err := uc.outboxRepo.WithTx(tx).CreateEvent(ctx, outboxEvent); err != nil {
			return fmt.Errorf("create outbox event for clarification answered: %w", err)
		}

		if input.PublishAsAnnouncement {
			title := input.AnnouncementTitle
			if title == "" {
				if clarification.ProblemLetter != nil {
					title = fmt.Sprintf("Вопрос по задаче %s", *clarification.ProblemLetter)
				} else {
					title = "Ответ на вопрос участников"
				}
			}

			body := fmt.Sprintf("**Вопрос:** %s\n\n**Ответ:** %s", clarification.Question, input.Answer)
			announcement, err := uc.announcementsRepo.WithTx(tx).CreateAnnouncement(ctx, &models.CreateContestAnnouncementInput{
				ContestID: clarification.ContestID,
				ProblemID: clarification.ProblemID,
				AuthorID:  input.AnsweredBy,
				Title:     title,
				Body:      body,
			})
			if err != nil {
				return fmt.Errorf("publish clarification as announcement: %w", err)
			}

			annEvent := models.ContestAnnouncementCreatedEvent{
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

			annPayload, err := json.Marshal(annEvent)
			if err != nil {
				return fmt.Errorf("marshal announcement event: %w", err)
			}

			annHeaders := make(map[string]string)
			otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(annHeaders))

			annOutboxEvent := &models.CreateOutboxEventParams{
				Id:          uuid.New(),
				AggregateID: announcement.ContestID,
				EventType:   models.OutboxEventContestAnnouncementCreated,
				Payload:     annPayload,
				Headers:     annHeaders,
			}

			if err := uc.outboxRepo.WithTx(tx).CreateEvent(ctx, annOutboxEvent); err != nil {
				return fmt.Errorf("create outbox event for announcement: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return clarification, nil
}
