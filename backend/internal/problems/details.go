package problems

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (uc *ProblemsUseCase) GetProblemDetails(ctx context.Context, id uuid.UUID) (*problemsmodel.ProblemDetails, error) {
	if err := authorizeProblem(ctx, uc.permissions, id, accessmodel.ActionViewProblem); err != nil {
		return nil, err
	}
	problem, err := uc.repo.GetProblemById(ctx, id)
	if err != nil {
		return nil, err
	}
	draft, err := uc.drafts.GetDraft(ctx, id)
	if err != nil {
		return nil, err
	}
	return &problemsmodel.ProblemDetails{Problem: problem, Statement: statementFromDraft(draft, "")}, nil
}
