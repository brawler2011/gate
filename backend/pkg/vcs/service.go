package vcs

import (
	"context"

	"github.com/google/uuid"
)

// Service defines workshop file storage operations.
type Service interface {
	CreateDirectory(ctx context.Context, problemID uuid.UUID, path string) error
	DeleteProblemWorkspace(ctx context.Context, problemID uuid.UUID) error

	ReadFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error)
	WriteFile(ctx context.Context, problemID uuid.UUID, path string, content []byte) error
	DeleteFile(ctx context.Context, problemID uuid.UUID, path string) error
	ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]FileEntry, error)
	ListAllFiles(ctx context.Context, problemID uuid.UUID) ([]string, error)
}
