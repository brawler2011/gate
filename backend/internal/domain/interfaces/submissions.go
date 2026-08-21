package interfaces

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SubmissionsRepo interface {
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error)
	ListSubmissions(ctx context.Context, filter models.SubmissionsFilter) ([]models.Submission, int32, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	BlockSubmission(ctx context.Context, id uuid.UUID, reason *string) error
	BlockUserProblemSubmissions(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string) error
	ResetSubmissionsState(ctx context.Context, filter models.RejudgeFilter) ([]uuid.UUID, error)
	WithTx(tx pgx.Tx) SubmissionsRepo
}

type SubmissionsUC interface {
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error)
	ListSubmissions(ctx context.Context, filter models.SubmissionsFilter) (*models.SubmissionsList, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	BlockSubmission(ctx context.Context, contestID, submissionID uuid.UUID, reason *string) error
	UnblockSubmission(ctx context.Context, contestID, submissionID uuid.UUID) error
	RejudgeSubmissions(ctx context.Context, filter models.RejudgeFilter) (int, error)
}
