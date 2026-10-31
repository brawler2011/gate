package usecase

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZipDirectoryPreservesPathsAndFileBytes(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"problem.yaml":          []byte("title: Test\n"),
		"tests/nested/edge.in":  {0, 255, 128, 10},
		"tests/nested/edge.ans": []byte("42\n"),
		"tests/empty.in":        {},
	}
	for name, data := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, data, 0600))
	}
	var buffer bytes.Buffer
	require.NoError(t, ZipDirectory(dir, &buffer))
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	require.NoError(t, err)
	require.Len(t, archive.File, len(files))
	actual := make(map[string][]byte)
	for _, file := range archive.File {
		reader, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		actual[file.Name] = data
	}
	require.Equal(t, files, actual)
}
