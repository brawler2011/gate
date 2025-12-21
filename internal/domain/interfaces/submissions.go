package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

type SubmissionsRepo interface {
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error)
	GetUntestedSubmissions(ctx context.Context, limit int32) ([]models.Submission, error)
	ListSubmissions(ctx context.Context, filter models.SubmissionsFilter) ([]models.Submission, int32, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
}

type SubmissionsUC interface {
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error)
	ListSubmissions(ctx context.Context, filter models.SubmissionsFilter) (*models.SubmissionsList, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
}
