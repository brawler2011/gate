package interfaces

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
)

type WorkshopUC interface {
	IsInitialized(ctx context.Context, problemID uuid.UUID) bool
	InitProblemWorkshop(ctx context.Context, problemID uuid.UUID, title string) error
	UpdateProblemFile(ctx context.Context, req models.UpdateFileRequest) error
	DeleteProblemFile(ctx context.Context, problemID uuid.UUID, path string) error
	ReadProblemFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error)
	ListProblemFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]vcs.FileEntry, error)
	CommitChanges(ctx context.Context, problemID uuid.UUID, message, authorName, authorEmail string) (string, error)
	GetWorkshopStatus(ctx context.Context, problemID uuid.UUID) (*models.WorkshopStatus, error)
	GetCommitHistory(ctx context.Context, problemID uuid.UUID, limit int) ([]vcs.Commit, error)
	CompileProblemComponent(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error)
	GenerateTests(ctx context.Context, req models.GenerateTestsRequest) error
	ValidateAllTests(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*models.ValidationReport, error)
	TestSolution(ctx context.Context, req models.TestSolutionRequest) (*models.TestReport, error)
}
