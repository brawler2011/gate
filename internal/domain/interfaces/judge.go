package interfaces

import (
	"context"

	"github.com/google/uuid"
)

// JudgeUseCase defines the interface for judging submissions
type JudgeUseCase interface {
	JudgeSubmission(ctx context.Context, submissionID uuid.UUID) error
}
