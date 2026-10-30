package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

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
