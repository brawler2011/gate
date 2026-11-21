package contests

import (
	"context"
	"fmt"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/pkg"
)

type ClarificationsUseCase struct {
	transactor  ContestEventsTransactions
	reads       ClarificationReads
	permissions Permissions
}

func NewClarificationsUseCase(transactor ContestEventsTransactions, reads ClarificationReads, permissions Permissions) *ClarificationsUseCase {
	return &ClarificationsUseCase{transactor: transactor, reads: reads, permissions: permissions}
}

func (uc *ClarificationsUseCase) CreateClarification(ctx context.Context, input *contestsmodel.CreateContestClarificationInput) (*contestsmodel.ContestClarification, error) {
	if input.Question == "" {
		return nil, pkg.AtField("/question", pkg.Invalid(pkg.ErrRequired, "question cannot be empty", nil))
	}

	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	var clarification *contestsmodel.ContestClarification
	err := withinContestEvents(ctx, uc.transactor, input.ContestID, accessmodel.ActionGetContest, func(tx ContestEventsTxStore) error {
		contest, err := tx.GetContest(ctx, input.ContestID)
		if err != nil {
			return err
		}

		settings := contestsmodel.MapToContestSettings(contest.Settings)
		if settings.AllowClarifications != nil && !*settings.AllowClarifications {
			return pkg.Wrap(pkg.ErrForbidden, nil, "clarifications are disabled for this contest")
		}

		clarification, err = tx.CreateClarification(ctx, user.Id, input)
		if err != nil {
			return err
		}

		event := contestsmodel.ContestClarificationCreatedEvent{
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

		outboxEvent := eventdata.Event{
			AggregateID: clarification.ContestID,
			Type:        eventdata.OutboxEventContestClarificationCreated,
			Payload:     event,
		}

		return tx.Record(ctx, outboxEvent)
	})
	if err != nil {
		return nil, err
	}

	return clarification, nil
}

func (uc *ClarificationsUseCase) AnswerClarification(ctx context.Context, input *contestsmodel.AnswerContestClarificationInput) (*contestsmodel.ContestClarification, error) {
	if input.Answer == "" {
		return nil, pkg.AtField("/answer", pkg.Invalid(pkg.ErrRequired, "answer cannot be empty", nil))
	}

	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}
	var clarification *contestsmodel.ContestClarification
	err := withinContestEvents(ctx, uc.transactor, input.ContestID, accessmodel.ActionManageContest, func(tx ContestEventsTxStore) error {
		var err error
		clarification, err = tx.AnswerClarification(
			ctx,
			input.ClarificationID,
			input.ContestID,
			user.Id,
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

		event := contestsmodel.ContestClarificationAnsweredEvent{
			ID:                 clarification.ID,
			ContestID:          clarification.ContestID,
			ProblemID:          clarification.ProblemID,
			ProblemTitle:       clarification.ProblemTitle,
			ProblemLetter:      clarification.ProblemLetter,
			UserID:             clarification.UserID,
			Username:           clarification.Username,
			Question:           clarification.Question,
			Answer:             input.Answer,
			AnsweredBy:         user.Id,
			AnsweredByUsername: ansByUsername,
			AnsweredAt:         ansAt,
		}

		outboxEvent := eventdata.Event{
			AggregateID: clarification.ContestID,
			Type:        eventdata.OutboxEventContestClarificationAnswered,
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
			announcement, err := tx.CreateAnnouncement(ctx, user.Id, &contestsmodel.CreateContestAnnouncementInput{
				ContestID: clarification.ContestID,
				ProblemID: clarification.ProblemID,
				Title:     title,
				Body:      body,
			})
			if err != nil {
				return fmt.Errorf("publish clarification as announcement: %w", err)
			}

			annEvent := contestsmodel.ContestAnnouncementCreatedEvent{
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

			annOutboxEvent := eventdata.Event{
				AggregateID: announcement.ContestID,
				Type:        eventdata.OutboxEventContestAnnouncementCreated,
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
