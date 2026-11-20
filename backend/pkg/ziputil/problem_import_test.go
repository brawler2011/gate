package ziputil

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/stretchr/testify/require"
)

func archive(t *testing.T, corrupt bool) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.CreateHeader(&zip.FileHeader{Name: "tests/1", Method: zip.Store})
	require.NoError(t, err)
	_, err = f.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	data := buf.Bytes()
	if corrupt {
		i := bytes.Index(data, []byte("payload"))
		data[i] = 'X'
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	return r
}

func TestExtractZipLimitedCodes(t *testing.T) {
	require.NoError(t, ExtractZipLimited(archive(t, false), t.TempDir(), 1024, 10))
	require.Equal(t, pkg.CodeInvalid, pkg.CodeOf(ExtractZipLimited(archive(t, true), t.TempDir(), 1024, 10)))
	require.Equal(t, pkg.CodePayloadTooLarge, pkg.CodeOf(ExtractZipLimited(archive(t, false), t.TempDir(), 3, 10)))
	require.Equal(t, pkg.CodePayloadTooLarge, pkg.CodeOf(ExtractZipLimited(archive(t, false), t.TempDir(), 1024, 1)))
}
