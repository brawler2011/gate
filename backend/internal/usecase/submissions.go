package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SubmissionsUseCase struct {
	submissionsRepo interfaces.SubmissionsRepo
	contestsUC      interfaces.ContestsUC
	problemsUC      interfaces.ProblemsUC
	outboxRepo      interfaces.OutboxRepo
	transactor      interfaces.Transactor
}

func NewSubmissionsUseCase(
	submissionsRepo interfaces.SubmissionsRepo,
	contestsUC interfaces.ContestsUC,
	problemsUC interfaces.ProblemsUC,
	outboxRepo interfaces.OutboxRepo,
	transactor interfaces.Transactor,
) *SubmissionsUseCase {
	return &SubmissionsUseCase{
		submissionsRepo: submissionsRepo,
		contestsUC:      contestsUC,
		problemsUC:      problemsUC,
		outboxRepo:      outboxRepo,
		transactor:      transactor,
	}
}

func (uc *SubmissionsUseCase) GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error) {
	return uc.submissionsRepo.GetSubmission(ctx, id)
}

func (uc *SubmissionsUseCase) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	_, err := uc.contestsUC.GetContest(ctx, creation.ContestId)
	if err != nil {
		return uuid.Nil, err
	}

	_, err = uc.problemsUC.GetProblemById(ctx, creation.ProblemId)
	if err != nil {
		return uuid.Nil, err
	}

	if creation.ContestId != uuid.Nil {
		_, err = uc.contestsUC.GetContestProblem(ctx, models.ContestProblemGet{
			ContestId: creation.ContestId,
			ProblemId: creation.ProblemId,
		})
		if err != nil {
			return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "problem does not belong to contest")
		}

		block, err := uc.contestsUC.GetProblemBlockStatusForUser(ctx, creation.ContestId, creation.UserId, creation.ProblemId)
		if err == nil && block != nil {
			dqState := models.Disqualified
			creation.State = &dqState
			creation.BanReason = block.Reason

			id, err := uc.submissionsRepo.CreateSubmission(ctx, creation)
			if err != nil {
				return uuid.Nil, err
			}

			submission, err := uc.submissionsRepo.GetSubmission(ctx, id)
			if err == nil && uc.contestsUC != nil {
				_ = uc.contestsUC.ProcessSubmissionResult(ctx, &submission)
			}
			slog.InfoContext(ctx, "submission_accepted", "event", "submission_accepted", "submission_id", id, "request_id", requestid.Get(ctx), "state", models.Disqualified)
			return id, nil
		}
	}

	var id uuid.UUID

	err = uc.transactor.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		id, err = uc.submissionsRepo.WithTx(tx).CreateSubmission(ctx, creation)
		if err != nil {
			return err
		}

		submission, err := uc.submissionsRepo.WithTx(tx).GetSubmission(ctx, id)
		if err != nil {
			return err
		}

		eventParams, err := newOutboxEventParams(ctx, submission, nil)
		if err != nil {
			return err
		}

		if err := uc.outboxRepo.WithTx(tx).CreateEvent(ctx, eventParams); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	slog.InfoContext(ctx, "submission_accepted", "event", "submission_accepted", "submission_id", id, "request_id", requestid.Get(ctx))
	return id, nil
}

func (uc *SubmissionsUseCase) BlockSubmission(ctx context.Context, contestID, submissionID uuid.UUID, reason *string) error {
	sub, err := uc.submissionsRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		return err
	}

	if contestID != uuid.Nil && sub.ContestID != nil && *sub.ContestID != contestID {
		return pkg.Wrap(pkg.ErrBadInput, nil, "submission does not belong to specified contest")
	}

	if err := uc.submissionsRepo.BlockSubmission(ctx, submissionID, reason); err != nil {
		return err
	}

	sub.State = models.Disqualified
	sub.BanReason = reason
	if uc.contestsUC != nil {
		_ = uc.contestsUC.ProcessSubmissionResult(ctx, &sub)
	}

	return nil
}

func (uc *SubmissionsUseCase) UnblockSubmission(ctx context.Context, contestID, submissionID uuid.UUID) error {
	filter := models.RejudgeFilter{
		ContestID:    contestID,
		SubmissionID: &submissionID,
	}
	_, err := uc.RejudgeSubmissions(ctx, filter)
	return err
}

func newOutboxEventParams(ctx context.Context, submission models.Submission, previousState *models.State) (*models.CreateOutboxEventParams, error) {
	submissionCreatedEvent := models.SubmissionCreatedEvent{
		IsNew: previousState == nil, PreviousState: previousState,
		SubmissionEventMeta: models.SubmissionEventMeta{
			UserId:       submission.CreatedBy,
			Username:     submission.Username,
			ContestId:    submission.ContestID,
			ContestTitle: submission.ContestTitle,
			ProblemId:    submission.ProblemID,
			ProblemTitle: submission.ProblemTitle,
			Position:     submission.Position,
			Language:     submission.Language,
			CreatedAt:    submission.CreatedAt,
			UpdatedAt:    submission.UpdatedAt,
		},
		Id:     submission.ID,
		State:  submission.State,
		Source: submission.Submission,
	}

	payload, err := json.Marshal(submissionCreatedEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	headers := make(map[string]string)
	headers[requestid.Header] = requestid.Get(ctx)

	eventParams := &models.CreateOutboxEventParams{
		Id:          uuid.New(),
		AggregateID: submission.ID,
		EventType:   models.OutboxEventSubmissionCreated,
		Payload:     payload,
		Headers:     headers,
	}

	return eventParams, nil
}

func (uc *SubmissionsUseCase) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	return uc.submissionsRepo.UpdateSubmission(ctx, id, update)
}

func (uc *SubmissionsUseCase) ListSubmissions(ctx context.Context, filter models.SubmissionsFilter) (*models.SubmissionsList, error) {
	submissions, total, err := uc.submissionsRepo.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &models.SubmissionsList{
		Submissions: submissions,
		Pagination:  models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *SubmissionsUseCase) RejudgeSubmissions(ctx context.Context, filter models.RejudgeFilter) (int, error) {
	if filter.ContestID == uuid.Nil {
		return 0, pkg.Wrap(pkg.ErrBadInput, nil, "contest id is required")
	}

	_, err := uc.contestsUC.GetContest(ctx, filter.ContestID)
	if err != nil {
		return 0, err
	}

	if filter.ProblemID != nil && *filter.ProblemID != uuid.Nil {
		_, err = uc.problemsUC.GetProblemById(ctx, *filter.ProblemID)
		if err != nil {
			return 0, err
		}
	}

	if filter.SubmissionID != nil && *filter.SubmissionID != uuid.Nil {
		sub, err := uc.submissionsRepo.GetSubmission(ctx, *filter.SubmissionID)
		if err != nil {
			return 0, err
		}
		if sub.ContestID != nil && *sub.ContestID != filter.ContestID {
			return 0, pkg.Wrap(pkg.ErrBadInput, nil, "submission does not belong to specified contest")
		}
	}

	var rejudgedCount int
	var rejudgedIDs []uuid.UUID

	err = uc.transactor.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		txSubRepo := uc.submissionsRepo.WithTx(tx)
		txOutboxRepo := uc.outboxRepo.WithTx(tx)

		resetIDs, err := txSubRepo.ResetSubmissionsState(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to reset submissions state: %w", err)
		}

		for _, reset := range resetIDs {
			id := reset.ID
			submission, err := txSubRepo.GetSubmission(ctx, id)
			if err != nil {
				return fmt.Errorf("failed to get submission %s for rejudge: %w", id, err)
			}

			eventParams, err := newOutboxEventParams(ctx, submission, &reset.PreviousState)
			if err != nil {
				return fmt.Errorf("failed to create event params for submission %s: %w", id, err)
			}

			if err := txOutboxRepo.CreateEvent(ctx, eventParams); err != nil {
				return fmt.Errorf("failed to create outbox event for submission %s: %w", id, err)
			}
		}

		rejudgedCount = len(resetIDs)
		for _, reset := range resetIDs {
			rejudgedIDs = append(rejudgedIDs, reset.ID)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	for _, id := range rejudgedIDs {
		slog.InfoContext(ctx, "submission_rejudge_requested", "event", "submission_rejudge_requested", "submission_id", id, "request_id", requestid.Get(ctx))
	}
	return rejudgedCount, nil
}
