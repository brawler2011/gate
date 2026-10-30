package usecase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type WorkspaceStorage struct {
	storage storage.Storage
	bucket  string
	locker  interfaces.WorkspaceLocker
}

func NewWorkspaceStorage(storage storage.Storage, bucket string, locker interfaces.WorkspaceLocker) *WorkspaceStorage {
	return &WorkspaceStorage{
		storage: storage,
		bucket:  bucket,
		locker:  locker,
	}
}

func (w *WorkspaceStorage) getWorkspaceKey(problemID uuid.UUID, path string) string {
	return fmt.Sprintf("workspaces/%s/%s", problemID.String(), path)
}

func (w *WorkspaceStorage) ReadFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	key := w.getWorkspaceKey(problemID, path)
	rc, _, err := w.storage.DownloadFile(ctx, w.bucket, key, nil)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspace file content: %w", err)
	}
	return data, nil
}

func (w *WorkspaceStorage) writeFileUnlocked(ctx context.Context, problemID uuid.UUID, path string, content []byte) error {
	key := w.getWorkspaceKey(problemID, path)
	return w.storage.UploadFile(ctx, w.bucket, key, bytes.NewReader(content), "application/octet-stream")
}

func (w *WorkspaceStorage) deleteFileUnlocked(ctx context.Context, problemID uuid.UUID, path string) error {
	key := w.getWorkspaceKey(problemID, path)
	return w.storage.DeleteFile(ctx, w.bucket, key)
}

func (w *WorkspaceStorage) ListAllObjects(ctx context.Context, problemID uuid.UUID) ([]storage.ObjectInfo, error) {
	prefix := fmt.Sprintf("workspaces/%s/", problemID.String())
	objects, err := w.storage.ListObjects(ctx, w.bucket, prefix)
	if err != nil {
		return nil, err
	}
	var filtered []storage.ObjectInfo
	for _, obj := range objects {
		rel := strings.TrimPrefix(obj.Key, prefix)
		if rel != "" {
			parts := strings.Split(rel, "/")
			isDotfile := false
			for _, p := range parts {
				if strings.HasPrefix(p, ".") {
					isDotfile = true
					break
				}
			}
			if !isDotfile {
				filtered = append(filtered, storage.ObjectInfo{
					Key:  rel,
					Size: obj.Size,
				})
			}
		}
	}
	return filtered, nil
}

func (w *WorkspaceStorage) ListAllFiles(ctx context.Context, problemID uuid.UUID) ([]string, error) {
	objects, err := w.ListAllObjects(ctx, problemID)
	if err != nil {
		return nil, err
	}
	files := make([]string, len(objects))
	for i, obj := range objects {
		files[i] = obj.Key
	}
	return files, nil
}

func (w *WorkspaceStorage) ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]models.FileEntry, error) {
	allObjects, err := w.ListAllObjects(ctx, problemID)
	if err != nil {
		return nil, err
	}

	prefix := ""
	if dirPath != "" && dirPath != "." {
		prefix = strings.TrimSuffix(dirPath, "/") + "/"
	}

	seen := make(map[string]bool)
	var entries []models.FileEntry

	for _, obj := range allObjects {
		file := obj.Key
		if prefix != "" && !strings.HasPrefix(file, prefix) {
			continue
		}
		rel := strings.TrimPrefix(file, prefix)
		parts := strings.Split(rel, "/")

		if len(parts) == 0 || parts[0] == "" {
			continue
		}

		name := parts[0]
		if seen[name] {
			continue
		}
		seen[name] = true

		isDir := len(parts) > 1
		var entryPath string
		if prefix != "" {
			entryPath = prefix + name
		} else {
			entryPath = name
		}

		size := obj.Size
		if isDir {
			size = 0
		}

		entries = append(entries, models.FileEntry{
			Path:        entryPath,
			IsDirectory: isDir,
			Size:        size,
		})
	}

	return entries, nil
}

func (w *WorkspaceStorage) deleteProblemWorkspaceUnlocked(ctx context.Context, problemID uuid.UUID) error {
	files, err := w.ListAllFiles(ctx, problemID)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := w.DeleteFile(ctx, problemID, file); err != nil {
			return fmt.Errorf("delete workspace file %s: %w", file, err)
		}
	}
	return nil
}

// WorkspaceMutex is a process-local locker for standalone tools and tests.
// The server supplies a PostgreSQL locker shared by all server instances.
type WorkspaceMutex struct{ mu sync.Mutex }

func (m *WorkspaceMutex) WithWorkspaceLock(ctx context.Context, _ uuid.UUID, fn func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(ctx)
}

type workspaceLockKey struct {
	storage *WorkspaceStorage
	id      uuid.UUID
}

func (w *WorkspaceStorage) withLock(ctx context.Context, id uuid.UUID, fn func(context.Context) error) error {
	key := workspaceLockKey{w, id}
	if ctx.Value(key) != nil {
		return fn(ctx)
	}
	return w.locker.WithWorkspaceLock(ctx, id, func(ctx context.Context) error {
		return fn(context.WithValue(ctx, key, true))
	})
}

func withWorkspaceValue[T any](ctx context.Context, w *WorkspaceStorage, id uuid.UUID, fn func(context.Context) (T, error)) (result T, err error) {
	err = w.withLock(ctx, id, func(ctx context.Context) error {
		result, err = fn(ctx)
		return err
	})
	return result, err
}

func (w *WorkspaceStorage) WriteFile(ctx context.Context, problemID uuid.UUID, path string, content []byte) error {
	return w.withLock(ctx, problemID, func(ctx context.Context) error { return w.writeFileUnlocked(ctx, problemID, path, content) })
}

func (w *WorkspaceStorage) DeleteFile(ctx context.Context, problemID uuid.UUID, path string) error {
	return w.withLock(ctx, problemID, func(ctx context.Context) error { return w.deleteFileUnlocked(ctx, problemID, path) })
}

func (w *WorkspaceStorage) DeleteProblemWorkspace(ctx context.Context, problemID uuid.UUID) error {
	return w.withLock(ctx, problemID, func(ctx context.Context) error { return w.deleteProblemWorkspaceUnlocked(ctx, problemID) })
}
