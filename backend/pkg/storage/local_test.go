package storage

import (
	"bytes"
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalUploadFailurePreservesPreviousObject(t *testing.T) {
	ctx := context.Background()
	store := NewLocalStorage(t.TempDir())
	require.NoError(t, store.UploadFile(ctx, "bucket", "package.zip", strings.NewReader("complete object"), "application/zip"))
	original, _, err := store.DownloadFile(ctx, "bucket", "package.zip", nil)
	require.NoError(t, err)
	defer original.Close()
	failure := errors.New("interrupted reader")
	reader := io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(failure))
	require.ErrorIs(t, store.UploadFile(ctx, "bucket", "package.zip", reader, "application/zip"), failure)
	fresh, _, err := store.DownloadFile(ctx, "bucket", "package.zip", nil)
	require.NoError(t, err)
	data, err := io.ReadAll(fresh)
	require.NoError(t, err)
	require.NoError(t, fresh.Close())
	require.Equal(t, "complete object", string(data))
	require.NoError(t, store.UploadFile(ctx, "bucket", "package.zip", strings.NewReader("replacement"), "application/zip"))
	data, err = io.ReadAll(original)
	require.NoError(t, err)
	require.Equal(t, "complete object", string(data), "an existing reader keeps its complete old object")
	keys, err := store.ListFiles(ctx, "bucket", "")
	require.NoError(t, err)
	require.Equal(t, []string{"package.zip"}, keys)
}

func TestLocalStorageOperations(t *testing.T) {
	ctx := context.Background()

	baseDir := t.TempDir()

	localStorage := NewLocalStorage(baseDir)

	bucket := "test-bucket"

	err := localStorage.EnsureBucket(ctx, bucket)
	require.NoError(t, err)

	testContent := []byte("Hello, Local FS!")
	testKey := "test-file.txt"
	err = localStorage.UploadFile(ctx, bucket, testKey, bytes.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	body, etag, err := localStorage.DownloadFile(ctx, bucket, testKey, nil)
	require.NoError(t, err)
	defer body.Close()

	downloadedContent, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, testContent, downloadedContent)
	assert.NotEmpty(t, etag)

	// Test ErrNotModified
	_, _, err = localStorage.DownloadFile(ctx, bucket, testKey, &etag)
	require.ErrorIs(t, err, ErrNotModified)

	keys, err := localStorage.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.Contains(t, keys, testKey)

	url, err := localStorage.GetPresignedURL(ctx, bucket, testKey, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "/api/files/test-bucket/test-file.txt", url)

	err = localStorage.DeleteFile(ctx, bucket, testKey)
	require.NoError(t, err)

	keys, err = localStorage.ListFiles(ctx, bucket, "")
	require.NoError(t, err)
	assert.NotContains(t, keys, testKey)
}
