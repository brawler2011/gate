package submissions

import (
	"context"
	"encoding/json"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type Repo interface {
	GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error)
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error)
}

type ContestsRepo interface {
	GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error)
}

type ProblemsRepo interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
}

type OutboxRepo interface {
	InsertEvent(ctx context.Context, event *models.OutboxEvent) error
}

type UseCase struct {
	solutionsRepo Repo
	contestsRepo  ContestsRepo
	problemsRepo  ProblemsRepo
	outboxRepo    OutboxRepo
}

func NewUseCase(
	solutionsRepo Repo,
	contestsRepo ContestsRepo,
	problemsRepo ProblemsRepo,
	outboxRepo OutboxRepo,
) *UseCase {
	return &UseCase{
		solutionsRepo: solutionsRepo,
		contestsRepo:  contestsRepo,
		problemsRepo:  problemsRepo,
		outboxRepo:    outboxRepo,
	}
}

func (uc *UseCase) GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error) {
	return uc.solutionsRepo.GetSubmissions(ctx, id)
}

func (uc *UseCase) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	// Validate contest exists
	_, err := uc.contestsRepo.GetContest(ctx, creation.ContestId)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "contest not found")
	}

	// Validate problem exists
	_, err = uc.problemsRepo.GetProblemById(ctx, creation.ProblemId)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "problem not found")
	}

	// Save submission to database (state will be Saved (1) by default)
	id, err := uc.solutionsRepo.CreateSubmission(ctx, creation)
	if err != nil {
		return uuid.Nil, err
	}

	// Create outbox event for submission.created
	payload := models.SubmissionCreatedPayload{
		SubmissionId: id,
		ProblemId:    creation.ProblemId,
		ContestId:    creation.ContestId,
		Language:     int64(creation.Language),
		CreatedBy:    creation.UserId,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to marshal outbox payload")
	}

	event := &models.OutboxEvent{
		AggregateId:   id,
		AggregateType: "submission",
		EventType:     models.EventTypeSubmissionCreated,
		Payload:       payloadBytes,
		Status:        models.OutboxEventStatusPending,
		RetryCount:    0,
	}

	if err := uc.outboxRepo.InsertEvent(ctx, event); err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to insert outbox event")
	}

	return id, nil
}

func (uc *UseCase) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	return uc.solutionsRepo.UpdateSubmission(ctx, id, update)
}

func (uc *UseCase) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error) {
	return uc.solutionsRepo.ListSolutions(ctx, filter)
}
