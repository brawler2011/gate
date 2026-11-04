package judgeworker

import (
	"context"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
)

// JudgeUseCase defines the interface for judging submissions
type JudgeUseCase interface {
	JudgeSubmission(ctx context.Context, reference judgingmodel.JobRef) error
}
