package problems

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type updateTransactions struct {
	ProblemTxStore
	active, updated bool
	calls           int
	commitError     error
	previous        string
	metadata        *problemsmodel.WorkspaceMetadata
}

func (s *updateTransactions) Within(_ context.Context, fn func(ProblemTxStore) error) error {
	s.active = true
	s.calls++
	defer func() { s.active = false }()
	return fn(s)
}
func (*updateTransactions) GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error) {
	return problemsmodel.Problem{}, nil
}
func (*updateTransactions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return true, nil
}
func (s *updateTransactions) CommitWorkspace(_ context.Context, _ uuid.UUID, previous, _ string, metadata *problemsmodel.WorkspaceMetadata) error {
	s.previous = previous
	s.metadata = metadata
	return s.commitError
}
func (s *updateTransactions) UpdateProblem(context.Context, uuid.UUID, *problemsmodel.ProblemUpdate) error {
	s.updated = true
	return nil
}

type updateWorkspaceRepo struct{ WorkspaceRepo }

func (updateWorkspaceRepo) GetWorkspaceHash(context.Context, uuid.UUID) (string, error) {
	return "previous", nil
}

type updateObjectStorage struct {
	storage.Storage
	transaction *updateTransactions
	archive     []byte
	failure     error
}

func (s updateObjectStorage) DownloadFile(context.Context, string, string, *string) (io.ReadCloser, string, error) {
	if s.transaction.active {
		return nil, "", errors.New("download inside SQL transaction")
	}
	return io.NopCloser(bytes.NewReader(s.archive)), "", nil
}
func (s updateObjectStorage) UploadFile(_ context.Context, _, _ string, reader io.Reader, _ string) error {
	if s.transaction.active {
		return errors.New("upload inside SQL transaction")
	}
	if s.failure != nil {
		return s.failure
	}
	_, err := io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("read uploaded candidate: %w", err)
	}
	return nil
}

func TestProblemUpdatePreparesObjectBeforeFencedCatalogChange(t *testing.T) {
	uploadFailure := errors.New("upload failed")
	for _, tc := range []struct {
		name                            string
		uploadError, commitError, error error
		updated                         bool
		calls                           int
	}{
		{name: "success", updated: true, calls: 1},
		{name: "upload failure", uploadError: uploadFailure, error: uploadFailure, calls: 0},
		{name: "workspace conflict", commitError: pkg.ErrConflict, error: pkg.ErrConflict, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &updateTransactions{commitError: tc.commitError}
			object := updateObjectStorage{transaction: tx, archive: createZipBytes(t, map[string]string{"problem.yaml": "title: Original\n"}), failure: tc.uploadError}
			workspace := NewWorkspaceStorage(object, "workspace", updateWorkspaceRepo{})
			uc := NewProblemsUseCase(nil, workspace, tx, nil, nil, allowProblemPermissions{})
			user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			title := "Changed"
			err := uc.UpdateProblem(accessmodel.WithUser(problemActorContext(), user), uuid.New(), &problemsmodel.ProblemUpdate{Title: &title})
			require.ErrorIs(t, err, tc.error)
			require.Equal(t, tc.calls, tx.calls)
			require.Equal(t, tc.updated, tx.updated)
			if tc.uploadError == nil {
				require.Equal(t, "previous", tx.previous)
				require.NotNil(t, tx.metadata)
				require.Equal(t, title, tx.metadata.Title)
			}
		})
	}
}
