package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotModified = errors.New("file not modified")
	ErrNotFound    = errors.New("file not found")
)

// Storage defines a unified interface for file operations across S3 and Local FS backends.
type Storage interface {
	UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error
	DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error)
	DeleteFile(ctx context.Context, bucket, key string) error
	ListFiles(ctx context.Context, bucket, prefix string) ([]string, error)
	GetPresignedURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error)
	EnsureBucket(ctx context.Context, bucket string) error
}
