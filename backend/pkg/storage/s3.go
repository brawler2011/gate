package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config holds configuration for S3 client
type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
}

// S3Storage implements the Storage interface for S3-compatible storage.
type S3Storage struct {
	client *s3.Client
}

// NewS3Storage creates a new S3 storage client
func NewS3Storage(cfg S3Config) *S3Storage {
	s3Client := s3.NewFromConfig(aws.Config{
		Region: cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		),
		RetryMaxAttempts: 5,
	}, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = true
	})

	return &S3Storage{
		client: s3Client,
	}
}

// UploadFile uploads a file to S3
func (s *S3Storage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	return s.uploadFile(ctx, bucket, key, reader, contentType, nil)
}

// UploadFileWithSHA256 requires S3 to validate the body against the expected
// hexadecimal SHA-256 before acknowledging the write.
func (s *S3Storage) UploadFileWithSHA256(ctx context.Context, bucket, key string, reader io.Reader, contentType, digest string) error {
	checksum, err := hex.DecodeString(digest)
	if err != nil || len(checksum) != sha256.Size {
		return fmt.Errorf("invalid upload SHA-256 %q", digest)
	}
	return s.uploadFile(ctx, bucket, key, reader, contentType, aws.String(base64.StdEncoding.EncodeToString(checksum)))
}

func (s *S3Storage) uploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string, checksum *string) error {
	// The SDK needs a seekable body for signing and retries. Callers already
	// holding a file avoid a copy; streams are spooled to disk, never the heap.
	body, ok := reader.(io.ReadSeeker)
	if !ok {
		file, err := os.CreateTemp("", "gate-s3-upload-*")
		if err != nil {
			return fmt.Errorf("create S3 spool: %w", err)
		}
		defer os.Remove(file.Name())
		defer file.Close()
		if _, err := io.Copy(file, reader); err != nil {
			return fmt.Errorf("spool S3 upload: %w", err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("rewind S3 spool: %w", err)
		}
		body = file
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:         aws.String(bucket),
		Key:            aws.String(key),
		Body:           body,
		ContentType:    aws.String(contentType),
		ChecksumSHA256: checksum,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}
	return nil
}

// DownloadFile downloads a file from S3.
// If ifNoneMatch matches, it returns ErrNotModified.
func (s *S3Storage) DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		IfNoneMatch: ifNoneMatch,
	})
	if err != nil {
		var re *http.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 304 {
			return nil, "", ErrNotModified
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NoSuchKey", "NotFound":
				return nil, "", ErrNotFound
			}
		}
		return nil, "", fmt.Errorf("failed to download file from S3: %w", err)
	}

	etag := ""
	if result.ETag != nil {
		etag = *result.ETag
	}
	return result.Body, etag, nil
}

// GetPresignedURL generates a presigned URL for accessing an S3 object
func (s *S3Storage) GetPresignedURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	presignedReq, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiration))

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedReq.URL, nil
}

// DeleteFile deletes a file from S3
func (s *S3Storage) DeleteFile(ctx context.Context, bucket, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}
	return nil
}

// ListObjects lists objects with metadata in an S3 bucket with the given prefix
func (s *S3Storage) ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	objects := make([]ObjectInfo, 0)
	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list S3 page: %w", err)
		}
		for _, obj := range result.Contents {
			if obj.Key != nil {
				size := int64(0)
				if obj.Size != nil {
					size = *obj.Size
				}
				objects = append(objects, ObjectInfo{
					Key:          *obj.Key,
					Size:         size,
					LastModified: aws.ToTime(obj.LastModified),
				})
			}
		}
	}

	return objects, nil
}

// ListFiles lists files in an S3 bucket with the given prefix
func (s *S3Storage) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
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

// ListBuckets returns the names of all buckets.
func (s *S3Storage) ListBuckets(ctx context.Context) ([]string, error) {
	paginator := s3.NewListBucketsPaginator(s.client, &s3.ListBucketsInput{})
	var names []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list S3 buckets: %w", err)
		}
		for _, bucket := range page.Buckets {
			names = append(names, aws.ToString(bucket.Name))
		}
	}
	return names, nil
}

// EmptyBucket deletes every object in the bucket and keeps the bucket.
func (s *S3Storage) EmptyBucket(ctx context.Context, bucket string) error {
	keys, err := s.ListFiles(ctx, bucket, "")
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := s.DeleteFile(ctx, bucket, key); err != nil {
			return err
		}
	}
	return nil
}

// BucketExists checks if an S3 bucket exists
func (s *S3Storage) BucketExists(ctx context.Context, bucket string) (bool, error) {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}

		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NotFound", "NoSuchBucket":
				return false, nil
			}
		}

		return false, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	return true, nil
}

// CreateBucket creates a new S3 bucket
func (s *S3Storage) CreateBucket(ctx context.Context, bucket string) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		ACL:    types.BucketCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	return nil
}

// EnsureBucket creates a bucket if it doesn't exist
func (s *S3Storage) EnsureBucket(ctx context.Context, bucket string) error {
	exists, err := s.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		return s.CreateBucket(ctx, bucket)
	}
	return nil
}
