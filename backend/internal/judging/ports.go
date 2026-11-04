package judging

import (
	"context"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type SubmissionsRepo interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionsmodel.Submission, error)
}
