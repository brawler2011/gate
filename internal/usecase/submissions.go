package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
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

		eventParams, err := newOutboxEventParams(submission)
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

	return id, nil
}

func newOutboxEventParams(submission models.Submission) (*models.CreateOutboxEventParams, error) {
	submissionCreatedEvent := models.SubmissionCreatedEvent{
		SubmissionEventMeta: models.SubmissionEventMeta{
			UserId:       submission.CreatedBy,
			Username:     submission.Username,
			ContestId:    submission.ContestID,
			ContestTitle: submission.ContestTitle,
			ProblemId:    submission.ProblemID,
			ProblemTitle: submission.ProblemTitle,
			Position:     submission.Position,
			Language:     submission.Language,
		},
		Id:     submission.ID,
		State:  submission.State,
		Source: submission.Submission,
	}

	payload, err := json.Marshal(submissionCreatedEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	eventParams := &models.CreateOutboxEventParams{
		Id:          uuid.New(),
		AggregateID: submission.ID,
		EventType:   models.OutboxEventSubmissionCreated,
		Payload:     payload,
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
