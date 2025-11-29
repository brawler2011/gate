package submissions

import (
	"context"
	"encoding/json"

	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	outboxsqlc "github.com/gate149/core/internal/outbox/sqlc"
	submissionssqlc "github.com/gate149/core/internal/submissions/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type Repo interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionssqlc.GetSubmissionRow, error)
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) ([]submissionssqlc.ListSubmissionsRow, int64, error)
}

type ContestsUC interface {
	GetContest(ctx context.Context, id uuid.UUID) (domain.Contest, error)
}

type ProblemsUC interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (domain.Problem, error)
}

type OutboxRepo interface {
	InsertEvent(ctx context.Context, event *outboxsqlc.OutboxEvent) error
}

type UseCase struct {
	solutionsRepo Repo
	contestsUC    ContestsUC
	problemsUC    ProblemsUC
	outboxRepo    OutboxRepo
}

func NewUseCase(
	solutionsRepo Repo,
	contestsUC ContestsUC,
	problemsUC ProblemsUC,
	outboxRepo OutboxRepo,
) *UseCase {
	return &UseCase{
		solutionsRepo: solutionsRepo,
		contestsUC:    contestsUC,
		problemsUC:    problemsUC,
		outboxRepo:    outboxRepo,
	}
}

func (uc *UseCase) GetSubmissions(ctx context.Context, id uuid.UUID) (domain.Submission, error) {
	s, err := uc.solutionsRepo.GetSubmission(ctx, id)
	if err != nil {
		return domain.Submission{}, err
	}
	return domain.SubmissionFromSqlc(s), nil
}

func (uc *UseCase) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	// Validate contest exists
	_, err := uc.contestsUC.GetContest(ctx, creation.ContestId)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "contest not found")
	}

	// Validate problem exists
	_, err = uc.problemsUC.GetProblemById(ctx, creation.ProblemId)
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

	event := &outboxsqlc.OutboxEvent{
		AggregateID:   id,
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

func (uc *UseCase) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*domain.SubmissionsList, error) {
	submissions, total, err := uc.solutionsRepo.ListSolutions(ctx, filter)
	if err != nil {
		return nil, err
	}

	domainSubmissions := make([]domain.Submission, len(submissions))
	for i, s := range submissions {
		domainSubmissions[i] = domain.SubmissionListRowFromSqlc(s)
	}

	return &domain.SubmissionsList{
		Submissions: domainSubmissions,
		Pagination:  domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}
