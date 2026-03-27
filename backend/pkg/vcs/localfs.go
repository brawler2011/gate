package vcs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/google/uuid"
)

type LocalFSService struct {
	baseDir string
	locks   sync.Map
}

// NewGoGitService is kept for compatibility in tests.
func NewGoGitService(baseDir string) *LocalFSService {
	return &LocalFSService{baseDir: baseDir}
}

func (s *LocalFSService) workspaceLock(problemID uuid.UUID) *sync.RWMutex {
	mu, _ := s.locks.LoadOrStore(problemID, &sync.RWMutex{})
	return mu.(*sync.RWMutex)
}

func (s *LocalFSService) workspacePath(problemID uuid.UUID) string {
	return filepath.Join(s.baseDir, problemID.String())
}

func (s *LocalFSService) absPath(problemID uuid.UUID, rel string) (string, error) {
	norm, err := normalizePath(rel)
	if err != nil {
		return "", err
	}
	if norm == "" {
		return "", fmt.Errorf("path is empty")
	}
	return filepath.Join(s.workspacePath(problemID), filepath.FromSlash(norm)), nil
}

func (s *LocalFSService) CreateDirectory(ctx context.Context, problemID uuid.UUID, path string) error {
	_ = ctx
	norm, err := normalizePath(path)
	if err != nil {
		return err
	}
	if norm == "" {
		return os.MkdirAll(s.workspacePath(problemID), 0o755)
	}

	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	return os.MkdirAll(filepath.Join(s.workspacePath(problemID), filepath.FromSlash(norm)), 0o755)
}

func (s *LocalFSService) DeleteProblemWorkspace(ctx context.Context, problemID uuid.UUID) error {
	_ = ctx
	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()
	return os.RemoveAll(s.workspacePath(problemID))
}

func (s *LocalFSService) ReadFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	_ = ctx
	abs, err := s.absPath(problemID, path)
	if err != nil {
		return nil, err
	}

	mu := s.workspaceLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	return os.ReadFile(abs)
}

func (s *LocalFSService) WriteFile(ctx context.Context, problemID uuid.UUID, path string, content []byte) error {
	_ = ctx
	abs, err := s.absPath(problemID, path)
	if err != nil {
		return err
	}

	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, content, 0o644)
}

func (s *LocalFSService) DeleteFile(ctx context.Context, problemID uuid.UUID, path string) error {
	_ = ctx
	abs, err := s.absPath(problemID, path)
	if err != nil {
		return err
	}

	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	return os.Remove(abs)
}

func (s *LocalFSService) ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]FileEntry, error) {
	_ = ctx
	norm, err := normalizePath(dirPath)
	if err != nil {
		return nil, err
	}

	root := s.workspacePath(problemID)
	if norm != "" {
		root = filepath.Join(root, filepath.FromSlash(norm))
	}

	mu := s.workspaceLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []FileEntry{}, nil
		}
		return nil, err
	}

	out := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		rel := entry.Name()
		if norm != "" {
			rel = filepath.ToSlash(filepath.Join(norm, rel))
		}
		out = append(out, FileEntry{Path: rel, IsDirectory: entry.IsDir(), Size: info.Size()})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDirectory != out[j].IsDirectory {
			return out[i].IsDirectory
		}
		return out[i].Path < out[j].Path
	})

	return out, nil
}

func (s *LocalFSService) ListAllFiles(ctx context.Context, problemID uuid.UUID) ([]string, error) {
	_ = ctx
	root := s.workspacePath(problemID)

	mu := s.workspaceLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	out := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	sort.Strings(out)
	return out, nil
}
