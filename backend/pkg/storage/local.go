package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage implements the Storage interface for local filesystem storage.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local filesystem storage provider
func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{
		basePath: basePath,
	}
}

// UploadFile writes reader content to a file at <basePath>/<bucket>/<key>
func (s *LocalStorage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	destPath := filepath.Join(s.basePath, bucket, filepath.Clean(key))

	// If it is a directory placeholder, create the directory and return
	if contentType == "application/x-directory" || strings.HasSuffix(key, "/") {
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory for local storage: %w", err)
		}
		return nil
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for local storage: %w", err)
	}

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, reader); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}

	return nil
}

// DownloadFile opens the file at <basePath>/<bucket>/<key>.
// Generates a weak ETag using file size and modification time.
// If ifNoneMatch matches, returns ErrNotModified.
func (s *LocalStorage) DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error) {
	srcPath := filepath.Join(s.basePath, bucket, filepath.Clean(key))

	info, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("failed to stat local file: %w", err)
	}

	// Generate weak ETag format: "size-modtime"
	etag := fmt.Sprintf(`W/"%x-%x"`, info.Size(), info.ModTime().UnixNano())

	if ifNoneMatch != nil && *ifNoneMatch == etag {
		return nil, "", ErrNotModified
	}

	file, err := os.Open(srcPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open local file: %w", err)
	}

	return file, etag, nil
}

// DeleteFile deletes the file at <basePath>/<bucket>/<key>
func (s *LocalStorage) DeleteFile(ctx context.Context, bucket, key string) error {
	targetPath := filepath.Join(s.basePath, bucket, filepath.Clean(key))

	if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("failed to delete local file: %w", removeErr)
	}

	// Clean up empty parent directories recursively
	bucketDir := filepath.Join(s.basePath, bucket)
	removeEmptyParents(filepath.Dir(targetPath), bucketDir, s.basePath)

	return nil
}

func removeEmptyParents(dir, bucketDir, basePath string) {
	for dir != bucketDir && dir != basePath && len(dir) > len(bucketDir) {
		if os.Remove(dir) != nil {
			break
		}
		dir = filepath.Dir(dir)
	}
}

// ListObjects lists all files with metadata in the bucket subdirectory matching prefix recursively
func (s *LocalStorage) ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	bucketDir := filepath.Join(s.basePath, bucket)

	if _, statErr := os.Stat(bucketDir); os.IsNotExist(statErr) {
		_ = statErr
		return nil, nil
	}

	var objects []ObjectInfo
	err := filepath.WalkDir(bucketDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != bucketDir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(bucketDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Normalize paths to forward slashes for unified key representation
		key := filepath.ToSlash(rel)
		if strings.HasPrefix(key, prefix) {
			size := int64(0)
			if info, err := d.Info(); err == nil {
				size = info.Size()
			}
			objects = append(objects, ObjectInfo{
				Key:  key,
				Size: size,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list local objects: %w", err)
	}

	return objects, nil
}

// ListFiles lists all files in the bucket subdirectory matching prefix recursively
func (s *LocalStorage) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	objects, err := s.ListObjects(ctx, bucket, prefix)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(objects))
	for i, obj := range objects {
		keys[i] = obj.Key
	}

	return keys, nil
}

// GetPresignedURL returns a direct HTTP URL to download/view the file via the file serving endpoint
func (s *LocalStorage) GetPresignedURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	// Normalize the key to forward slashes just in case
	urlKey := filepath.ToSlash(filepath.Clean(key))
	return fmt.Sprintf("/api/files/%s/%s", bucket, urlKey), nil
}

// EnsureBucket ensures the bucket directory exists
func (s *LocalStorage) EnsureBucket(ctx context.Context, bucket string) error {
	bucketDir := filepath.Join(s.basePath, bucket)
	if err := os.MkdirAll(bucketDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure local bucket directory: %w", err)
	}
	return nil
}
