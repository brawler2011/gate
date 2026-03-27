package vcs

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

type S3Service struct {
	s3Client *pkg.S3Client
	bucket   string
	locks    sync.Map
}

func NewS3Service(s3Client *pkg.S3Client, bucket string) *S3Service {
	return &S3Service{s3Client: s3Client, bucket: bucket}
}

func (s *S3Service) workspaceLock(problemID uuid.UUID) *sync.RWMutex {
	mu, _ := s.locks.LoadOrStore(problemID, &sync.RWMutex{})
	return mu.(*sync.RWMutex)
}

func (s *S3Service) workspacePrefix(problemID uuid.UUID) string {
	return fmt.Sprintf("workspaces/%s/", problemID.String())
}

func normalizePath(p string) (string, error) {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	p = strings.TrimPrefix(p, "/")
	p = path.Clean(p)
	if p == "." {
		return "", nil
	}
	if strings.HasPrefix(p, "../") || p == ".." {
		return "", fmt.Errorf("invalid path")
	}
	return p, nil
}

func (s *S3Service) objectKey(problemID uuid.UUID, p string) (string, error) {
	normalized, err := normalizePath(p)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		return "", fmt.Errorf("path is empty")
	}
	return s.workspacePrefix(problemID) + normalized, nil
}

func (s *S3Service) CreateDirectory(ctx context.Context, problemID uuid.UUID, p string) error {
	normalized, err := normalizePath(p)
	if err != nil {
		return err
	}
	if normalized == "" {
		return nil
	}
	if !strings.HasSuffix(normalized, "/") {
		normalized += "/"
	}

	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	key := s.workspacePrefix(problemID) + normalized
	return s.s3Client.UploadFile(ctx, s.bucket, key, strings.NewReader(""), "application/x-directory")
}

func (s *S3Service) DeleteProblemWorkspace(ctx context.Context, problemID uuid.UUID) error {
	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	keys, err := s.s3Client.ListFiles(ctx, s.bucket, s.workspacePrefix(problemID))
	if err != nil {
		return fmt.Errorf("failed to list workspace files: %w", err)
	}
	for _, key := range keys {
		if err := s.s3Client.DeleteFile(ctx, s.bucket, key); err != nil {
			return fmt.Errorf("failed to delete workspace file %s: %w", key, err)
		}
	}
	return nil
}

func (s *S3Service) ReadFile(ctx context.Context, problemID uuid.UUID, p string) ([]byte, error) {
	if strings.HasSuffix(strings.TrimSpace(p), "/") {
		return nil, fmt.Errorf("cannot read directory")
	}

	key, err := s.objectKey(problemID, p)
	if err != nil {
		return nil, err
	}

	mu := s.workspaceLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	obj, err := s.s3Client.DownloadFile(ctx, s.bucket, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", p, err)
	}
	defer obj.Body.Close()

	content, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content %s: %w", p, err)
	}

	return content, nil
}

func (s *S3Service) WriteFile(ctx context.Context, problemID uuid.UUID, p string, content []byte) error {
	if strings.HasSuffix(strings.TrimSpace(p), "/") {
		return fmt.Errorf("cannot write directory")
	}

	key, err := s.objectKey(problemID, p)
	if err != nil {
		return err
	}

	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	if err := s.s3Client.UploadFile(ctx, s.bucket, key, strings.NewReader(string(content)), "application/octet-stream"); err != nil {
		return fmt.Errorf("failed to write file %s: %w", p, err)
	}

	return nil
}

func (s *S3Service) DeleteFile(ctx context.Context, problemID uuid.UUID, p string) error {
	key, err := s.objectKey(problemID, p)
	if err != nil {
		return err
	}

	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	if err := s.s3Client.DeleteFile(ctx, s.bucket, key); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", p, err)
	}

	return nil
}

func (s *S3Service) ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]FileEntry, error) {
	normalizedDir, err := normalizePath(dirPath)
	if err != nil {
		return nil, err
	}

	mu := s.workspaceLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	keys, err := s.s3Client.ListFiles(ctx, s.bucket, s.workspacePrefix(problemID))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	entries := make(map[string]FileEntry)
	prefix := s.workspacePrefix(problemID)

	addEntry := func(name string, isDir bool) {
		if name == "" {
			return
		}
		entryPath := name
		if normalizedDir != "" {
			entryPath = normalizedDir + "/" + name
		}
		if existing, ok := entries[name]; ok {
			if existing.IsDirectory {
				return
			}
		}
		entries[name] = FileEntry{Path: entryPath, IsDirectory: isDir, Size: 0}
	}

	for _, key := range keys {
		rel := strings.TrimPrefix(key, prefix)
		if rel == "" {
			continue
		}

		if normalizedDir != "" {
			dirPrefix := normalizedDir + "/"
			if !strings.HasPrefix(rel, dirPrefix) {
				continue
			}
			rel = strings.TrimPrefix(rel, dirPrefix)
			if rel == "" {
				continue
			}
		}

		if strings.HasSuffix(rel, "/") {
			rel = strings.TrimSuffix(rel, "/")
			if rel == "" {
				continue
			}
			if strings.Contains(rel, "/") {
				name := strings.SplitN(rel, "/", 2)[0]
				addEntry(name, true)
				continue
			}
			addEntry(rel, true)
			continue
		}

		parts := strings.SplitN(rel, "/", 2)
		if len(parts) == 1 {
			addEntry(parts[0], false)
			continue
		}
		addEntry(parts[0], true)
	}

	if normalizedDir == "" {
		for _, dir := range []string{"tests", "solutions", "checkers", "validators", "generators", "interactors", "media"} {
			addEntry(dir, true)
		}
	}

	out := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDirectory != out[j].IsDirectory {
			return out[i].IsDirectory
		}
		return out[i].Path < out[j].Path
	})

	return out, nil
}

func (s *S3Service) ListAllFiles(ctx context.Context, problemID uuid.UUID) ([]string, error) {
	mu := s.workspaceLock(problemID)
	mu.RLock()
	defer mu.RUnlock()

	keys, err := s.s3Client.ListFiles(ctx, s.bucket, s.workspacePrefix(problemID))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	prefix := s.workspacePrefix(problemID)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		rel := strings.TrimPrefix(key, prefix)
		if rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		result = append(result, rel)
	}
	sort.Strings(result)
	return result, nil
}
