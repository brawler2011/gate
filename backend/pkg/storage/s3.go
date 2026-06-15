package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               cfg.Endpoint,
					SigningRegion:     cfg.Region,
					HostnameImmutable: true,
				}, nil
			},
		),
	}, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Storage{
		client: s3Client,
	}
}

// UploadFile uploads a file to S3
func (s *S3Storage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	all, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(all),
		ContentType: aws.String(contentType),
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

// ListFiles lists files in an S3 bucket with the given prefix
func (s *S3Storage) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files from S3: %w", err)
	}

	keys := make([]string, 0, len(result.Contents))
	for _, obj := range result.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}

	return keys, nil
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
