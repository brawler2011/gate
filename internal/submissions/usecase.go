package submissions

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
)

type Repo interface {
	GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error)
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error)
}

type UseCase struct {
	solutionsRepo Repo
}

func NewUseCase(
	solutionsRepo Repo,
) *UseCase {
	return &UseCase{
		solutionsRepo: solutionsRepo,
	}
}

func (uc *UseCase) GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error) {
	return uc.solutionsRepo.GetSubmissions(ctx, id)
}

func (uc *UseCase) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	panic("not implemented")
}

func (uc *UseCase) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	return uc.solutionsRepo.UpdateSubmission(ctx, id, update)
}

func (uc *UseCase) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error) {
	return uc.solutionsRepo.ListSolutions(ctx, filter)
}
