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

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{
		basePath: basePath,
	}
}

func (s *LocalStorage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	destPath := filepath.Join(s.basePath, bucket, filepath.Clean(key))

	if contentType == "application/x-directory" || strings.HasSuffix(key, "/") {
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory for local storage: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for local storage: %w", err)
	}

	destFile, err := os.CreateTemp(filepath.Dir(destPath), ".upload-*")
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer os.Remove(destFile.Name())

	if _, err := io.Copy(destFile, reader); err != nil {
		_ = destFile.Close()
		return fmt.Errorf("failed to write local file: %w", err)
	}
	if err := destFile.Close(); err != nil {
		return fmt.Errorf("close local object: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(destFile.Name(), destPath); err != nil {
		return fmt.Errorf("replace local object: %w", err)
	}
	return nil
}

// Local ETags are weak because they use size and modification time, not content.
func (s *LocalStorage) DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error) {
	srcPath := filepath.Join(s.basePath, bucket, filepath.Clean(key))

	info, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("failed to stat local file: %w", err)
	}

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

func (s *LocalStorage) DeleteFile(ctx context.Context, bucket, key string) error {
	targetPath := filepath.Join(s.basePath, bucket, filepath.Clean(key))

	if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("failed to delete local file: %w", removeErr)
	}

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

		// Object keys use forward slashes on every platform.
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

func (s *LocalStorage) GetPresignedURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	urlKey := filepath.ToSlash(filepath.Clean(key))
	return fmt.Sprintf("/api/files/%s/%s", bucket, urlKey), nil
}

func (s *LocalStorage) EnsureBucket(ctx context.Context, bucket string) error {
	bucketDir := filepath.Join(s.basePath, bucket)
	if err := os.MkdirAll(bucketDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure local bucket directory: %w", err)
	}
	return nil
}
