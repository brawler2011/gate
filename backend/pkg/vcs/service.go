package vcs

import (
	"context"

	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/google/uuid"
)

// Service defines the interface for VCS operations
type Service interface {
	// Repository lifecycle
	InitProblemRepo(ctx context.Context, problemID uuid.UUID) error
	DeleteProblemRepo(ctx context.Context, problemID uuid.UUID) error
	RepoExists(ctx context.Context, problemID uuid.UUID) bool

	// File operations
	ReadFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error)
	WriteFile(ctx context.Context, problemID uuid.UUID, path string, content []byte) error
	DeleteFile(ctx context.Context, problemID uuid.UUID, path string) error
	ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]FileEntry, error)

	// Git operations
	Commit(ctx context.Context, problemID uuid.UUID, message, authorName, authorEmail string) (commitSHA string, err error)
	GetStatus(ctx context.Context, problemID uuid.UUID) ([]FileStatus, error)
	GetHistory(ctx context.Context, problemID uuid.UUID, limit int) ([]Commit, error)
	GetCommitDiff(ctx context.Context, problemID uuid.UUID, commitSHA string) ([]FileDiff, error)
	GetCurrentSHA(ctx context.Context, problemID uuid.UUID) (string, error)
	HasUncommittedChanges(ctx context.Context, problemID uuid.UUID) (bool, error)

	// Problem format integration
	LoadManifest(ctx context.Context, problemID uuid.UUID) (*problemformat.ProblemManifest, error)
	SaveManifest(ctx context.Context, problemID uuid.UUID, manifest *problemformat.ProblemManifest) error
	ValidateRepoStructure(ctx context.Context, problemID uuid.UUID) error
	InitDefaultManifest(ctx context.Context, problemID uuid.UUID, title string) error
	InitDefaultTestsMetadata(ctx context.Context, problemID uuid.UUID) error

	// Repo metadata
	GetRepoPath(problemID uuid.UUID) string
}
