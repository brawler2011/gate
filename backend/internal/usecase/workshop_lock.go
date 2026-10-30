package usecase

import (
	"context"
	"path/filepath"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (uc *WorkshopUseCase) InitProblemWorkshop(ctx context.Context, problemID uuid.UUID, title string) error {
	return uc.workspaceStorage.withMutation(ctx, problemID, "initialize", func(ctx context.Context) error { return uc.initProblemWorkshopUnlocked(ctx, problemID, title) })
}

func (uc *WorkshopUseCase) UpdateProblemFile(ctx context.Context, req models.UpdateFileRequest) error {
	return uc.workspaceStorage.withMutation(ctx, req.ProblemID, "update:"+filepath.ToSlash(filepath.Clean(req.Path)), func(ctx context.Context) error { return uc.updateProblemFileUnlocked(ctx, req) })
}

func (uc *WorkshopUseCase) DeleteProblemFile(ctx context.Context, problemID uuid.UUID, path string) error {
	return uc.workspaceStorage.withMutation(ctx, problemID, "delete:"+filepath.ToSlash(filepath.Clean(path)), func(ctx context.Context) error { return uc.deleteProblemFileUnlocked(ctx, problemID, path) })
}

func (uc *WorkshopUseCase) CompileProblemComponent(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, req.ProblemID, func(ctx context.Context) (*models.CompileResult, error) {
		return uc.compileProblemComponentUnlocked(ctx, req)
	})
}

func (uc *WorkshopUseCase) loadManifest(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (*models.ProblemManifest, error) {
		return uc.loadManifestUnlocked(ctx, problemID)
	})
}

func (uc *WorkshopUseCase) saveManifest(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest) error {
	return uc.workspaceStorage.withLock(ctx, problemID, func(ctx context.Context) error { return uc.saveManifestUnlocked(ctx, problemID, manifest) })
}

func (uc *WorkshopUseCase) UpdateTestsConfig(ctx context.Context, problemID uuid.UUID, testsMeta *models.TestsMetadata) error {
	return uc.workspaceStorage.withLock(ctx, problemID, func(ctx context.Context) error { return uc.updateTestsConfigUnlocked(ctx, problemID, testsMeta) })
}

func (uc *WorkshopUseCase) GetTestsConfig(ctx context.Context, problemID uuid.UUID) (*models.TestsMetadata, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (*models.TestsMetadata, error) {
		return uc.getTestsConfigUnlocked(ctx, problemID)
	})
}

func (uc *WorkshopUseCase) ValidateAllTests(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*models.ValidationReport, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (*models.ValidationReport, error) {
		return uc.validateAllTestsUnlocked(ctx, problemID)
	})
}
func (uc *WorkshopUseCase) TestSolution(ctx context.Context, req models.TestSolutionRequest) (*models.TestReport, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, req.ProblemID, func(ctx context.Context) (*models.TestReport, error) { return uc.testSolutionUnlocked(ctx, req) })
}
