package vcs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/gate149/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type StorageClient interface {
	UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error
	DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error)
	DeleteFile(ctx context.Context, bucket, key string) error
	ListFiles(ctx context.Context, bucket, prefix string) ([]string, error)
}

type S3Service struct {
	storage StorageClient
	bucket  string
	locks   sync.Map
}

func NewS3Service(store storage.Storage, bucket string) *S3Service {
	return NewS3ServiceWithStorage(store, bucket)
}

func NewS3ServiceWithStorage(storage StorageClient, bucket string) *S3Service {
	return &S3Service{storage: storage, bucket: bucket}
}

func NewInMemoryS3Service(bucket string) *S3Service {
	return NewS3ServiceWithStorage(newInMemoryStorageClient(), bucket)
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
	return s.storage.UploadFile(ctx, s.bucket, key, strings.NewReader(""), "application/x-directory")
}

func (s *S3Service) DeleteProblemWorkspace(ctx context.Context, problemID uuid.UUID) error {
	mu := s.workspaceLock(problemID)
	mu.Lock()
	defer mu.Unlock()

	keys, err := s.storage.ListFiles(ctx, s.bucket, s.workspacePrefix(problemID))
	if err != nil {
		return fmt.Errorf("failed to list workspace files: %w", err)
	}
	for _, key := range keys {
		if err := s.storage.DeleteFile(ctx, s.bucket, key); err != nil {
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

	body, _, err := s.storage.DownloadFile(ctx, s.bucket, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", p, err)
	}
	defer body.Close()

	content, err := io.ReadAll(body)
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

	if err := s.storage.UploadFile(ctx, s.bucket, key, bytes.NewReader(content), "application/octet-stream"); err != nil {
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

	if err := s.storage.DeleteFile(ctx, s.bucket, key); err != nil {
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

	keys, err := s.storage.ListFiles(ctx, s.bucket, s.workspacePrefix(problemID))
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

	keys, err := s.storage.ListFiles(ctx, s.bucket, s.workspacePrefix(problemID))
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

type inMemoryStorageClient struct {
	mu      sync.RWMutex
	buckets map[string]map[string][]byte
}

func newInMemoryStorageClient() *inMemoryStorageClient {
	return &inMemoryStorageClient{buckets: make(map[string]map[string][]byte)}
}

func (c *inMemoryStorageClient) bucketObjects(bucket string) map[string][]byte {
	objects, ok := c.buckets[bucket]
	if !ok {
		objects = make(map[string][]byte)
		c.buckets[bucket] = objects
	}
	return objects
}

func (c *inMemoryStorageClient) existingBucketObjects(bucket string) map[string][]byte {
	objects, ok := c.buckets[bucket]
	if !ok {
		return nil
	}
	return objects
}

func (c *inMemoryStorageClient) UploadFile(_ context.Context, bucket, key string, reader io.Reader, _ string) error {
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	objects := c.bucketObjects(bucket)
	objects[key] = append([]byte(nil), content...)
	return nil
}

func (c *inMemoryStorageClient) DownloadFile(_ context.Context, bucket, key string, _ *string) (io.ReadCloser, string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	objects := c.existingBucketObjects(bucket)
	if objects == nil {
		return nil, "", fmt.Errorf("failed to download file from S3: object not found")
	}
	content, ok := objects[key]
	if !ok {
		return nil, "", fmt.Errorf("failed to download file from S3: object not found")
	}

	return io.NopCloser(bytes.NewReader(append([]byte(nil), content...))), "", nil
}

func (c *inMemoryStorageClient) DeleteFile(_ context.Context, bucket, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	objects := c.bucketObjects(bucket)
	delete(objects, key)
	return nil
}

func (c *inMemoryStorageClient) ListFiles(_ context.Context, bucket, prefix string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	objects := c.existingBucketObjects(bucket)
	if objects == nil {
		return []string{}, nil
	}
	keys := make([]string, 0, len(objects))
	for key := range objects {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
