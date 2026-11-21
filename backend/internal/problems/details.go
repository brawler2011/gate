package problems

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (uc *ProblemsUseCase) GetProblemDetails(ctx context.Context, id uuid.UUID) (*problemsmodel.ProblemDetails, error) {
	if _, err := access.Problem(ctx, uc.access, id, accessmodel.ProblemView); err != nil {
		return nil, err
	}
	problem, err := uc.repo.GetProblemById(ctx, id)
	if err != nil {
		return nil, err
	}
	return &problemsmodel.ProblemDetails{Problem: problem}, nil
}
