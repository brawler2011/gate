package problems

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (uc *ProblemsUseCase) GetProblemDetails(ctx context.Context, id uuid.UUID) (*problemsmodel.ProblemDetails, error) {
	if err := withinProblem(ctx, uc.transactions, id, accessmodel.ActionViewProblem, func(ProblemTxStore) error { return nil }); err != nil {
		return nil, err
	}
	problem, err := uc.repo.GetProblemById(ctx, id)
	if err != nil {
		return nil, err
	}
	result := &problemsmodel.ProblemDetails{Problem: problem}
	editor := newWorkshop(uc.workspace)
	err = uc.workspace.withPackage(ctx, id, func(ctx context.Context, _ *workspacePackage) error {
		initialized, err := editor.isInitialized(ctx, id)
		if err != nil {
			return err
		}
		if !initialized {
			return nil
		}
		manifest, err := editor.loadManifest(ctx, id)
		if err != nil {
			return err
		}
		result.Problem.Title = manifest.Statement.Title
		result.Problem.TimeLimitMs = manifest.TimeLimitMs
		result.Problem.MemoryLimitMb = manifest.MemoryLimitMb
		source := NewStatementReader(uc.workspace, nil)
		result.Statement, result.Samples = source.loadWorkspaceStatementAndSamples(ctx, id, "")
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
