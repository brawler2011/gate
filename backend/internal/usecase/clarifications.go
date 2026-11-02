package usecase

import (
	"context"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/events"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
)

type ClarificationsUseCase struct {
	clarificationsRepo interfaces.ClarificationsRepo
	announcementsRepo  interfaces.AnnouncementsRepo
	contestsUC         interfaces.ContestsUC
	outboxRepo         interfaces.OutboxRepo
	transactor         ContestEventsTransactions
}

func NewClarificationsUseCase(
	clarificationsRepo interfaces.ClarificationsRepo,
	announcementsRepo interfaces.AnnouncementsRepo,
	contestsUC interfaces.ContestsUC,
	outboxRepo interfaces.OutboxRepo,
	transactor ContestEventsTransactions,
) *ClarificationsUseCase {
	return &ClarificationsUseCase{
		clarificationsRepo: clarificationsRepo,
		announcementsRepo:  announcementsRepo,
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
	err = uc.transactor.Within(ctx, func(tx ContestEventsTxStore) error {
		var err error
		clarification, err = tx.CreateClarification(ctx, input)
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

		outboxEvent := events.Event{
			AggregateID: clarification.ContestID,
			Type:        models.OutboxEventContestClarificationCreated,
			Payload:     event,
		}

		return tx.Record(ctx, outboxEvent)
	})
	if err != nil {
		return nil, err
	}

	return clarification, nil
}

func (uc *ClarificationsUseCase) AnswerClarification(ctx context.Context, input *models.AnswerContestClarificationInput) (*models.ContestClarification, error) {
	if input.Answer == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "answer cannot be empty")
	}

	var clarification *models.ContestClarification
	err := uc.transactor.Within(ctx, func(tx ContestEventsTxStore) error {
		var err error
		clarification, err = tx.AnswerClarification(
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

		outboxEvent := events.Event{
			AggregateID: clarification.ContestID,
			Type:        models.OutboxEventContestClarificationAnswered,
			Payload:     event,
		}

		if err := tx.Record(ctx, outboxEvent); err != nil {
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
			announcement, err := tx.CreateAnnouncement(ctx, &models.CreateContestAnnouncementInput{
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

			annOutboxEvent := events.Event{
				AggregateID: announcement.ContestID,
				Type:        models.OutboxEventContestAnnouncementCreated,
				Payload:     annEvent,
			}

			if err := tx.Record(ctx, annOutboxEvent); err != nil {
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
