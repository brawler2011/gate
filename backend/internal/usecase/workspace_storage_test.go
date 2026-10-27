package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceStorage_ReadFile(t *testing.T) {
	readErr := errors.New("read failed")
	downloadErr := errors.New("download failed")
	for _, tt := range []struct {
		name        string
		reader      io.Reader
		downloadErr error
		wantErr     error
	}{
		{name: "success", reader: strings.NewReader("workspace content")},
		{name: "read failure", reader: iotest.ErrReader(readErr), wantErr: readErr},
		{name: "download failure", downloadErr: downloadErr, wantErr: downloadErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rc := &workspaceReadCloser{Reader: tt.reader}
			store := &workspaceDownloadStorage{reader: rc, err: tt.downloadErr}
			ws := NewWorkspaceStorage(store, "workspaces")

			data, err := ws.ReadFile(context.Background(), uuid.New(), "problem.yaml")

			if tt.downloadErr != nil {
				require.ErrorIs(t, err, downloadErr)
				require.Nil(t, data)
				require.Zero(t, rc.closeCalls)
				return
			}
			require.Equal(t, 1, rc.closeCalls)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.EqualError(t, err, "failed to read workspace file content: read failed")
				require.Nil(t, data)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []byte("workspace content"), data)
		})
	}
}

type workspaceReadCloser struct {
	io.Reader
	closeCalls int
}

func (r *workspaceReadCloser) Close() error {
	r.closeCalls++
	return nil
}

type workspaceDownloadStorage struct {
	storage.Storage
	reader io.ReadCloser
	err    error
}

func (s *workspaceDownloadStorage) DownloadFile(context.Context, string, string, *string) (io.ReadCloser, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return s.reader, "", nil
}
