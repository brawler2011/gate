//go:build integration
// +build integration

package integration

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3ClientOperations(t *testing.T) {
	ctx, s3Client := newSeaweedBackedS3Storage(t)

	bucket := "test-bucket-" + uuid.New().String()

	err := s3Client.EnsureBucket(ctx, bucket)
	require.NoError(t, err)

	testContent := []byte("Hello, S3!")
	testKey := "test-file.txt"
	err = s3Client.UploadFile(ctx, bucket, testKey, bytes.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	body, _, err := s3Client.DownloadFile(ctx, bucket, testKey, nil)
	require.NoError(t, err)
	defer body.Close()

	downloadedContent, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, testContent, downloadedContent)

	keys, err := s3Client.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.Contains(t, keys, testKey)

	url, err := s3Client.GetPresignedURL(ctx, bucket, testKey, 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, url)

	response, err := http.Get(url)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)

	err = s3Client.DeleteFile(ctx, bucket, testKey)
	require.NoError(t, err)

	keys, err = s3Client.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.NotContains(t, keys, testKey)
}
