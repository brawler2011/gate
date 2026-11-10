package problems

import (
	"context"
	"io"
	"os"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// recordingStore fails the test on any object-store access.
type recordingStore struct{ calls int }

func (s *recordingStore) PutImmutable(context.Context, string, string, *os.File, int64, string) error {
	s.calls++
	return nil
}

func (s *recordingStore) VerifyObject(context.Context, string, string, int64, string) error {
	s.calls++
	return nil
}

func (s *recordingStore) DownloadFile(context.Context, string, string, *string) (io.ReadCloser, string, error) {
	s.calls++
	return nil, "", nil
}

type savingRepo struct {
	DraftRepository
	saved []model.Draft
}

func (r *savingRepo) SaveDraft(_ context.Context, _ model.MutationKey, d model.Draft) (model.MutationResult, error) {
	r.saved = append(r.saved, d)
	return model.MutationResult{Status: 200}, nil
}

type singleProblem struct{ org uuid.UUID }

func (p singleProblem) GetProblemById(_ context.Context, id uuid.UUID) (model.Problem, error) {
	return model.Problem{ID: id, OrganizationID: p.org}, nil
}

// Editing metadata or text is a database-only operation: it never reads test bodies from the
// object store and never rebuilds an archive.
func TestSavingDraftTextNeverTouchesObjectStore(t *testing.T) {
	store := &recordingStore{}
	repo := &savingRepo{}
	problem := uuid.New()
	uc := &DraftUseCase{Repo: repo, Store: store, Problems: singleProblem{uuid.New()}, Permissions: allowProblemPermissions{}}
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})

	draft := model.Draft{ProblemID: problem, Revision: 4, Title: "Renamed", Files: map[string]string{"statements/en.md": "New statement"}}
	_, err := uc.SaveDraft(ctx, problem, 4, "key-1", draft)
	require.NoError(t, err)
	require.Len(t, repo.saved, 1)
	require.Equal(t, "Renamed", repo.saved[0].Title)
	require.Zero(t, store.calls)
}

type allowProblemPermissions struct{}

func (allowProblemPermissions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return true, nil
}

// The revision the client read must match the draft it sends, otherwise the request is
// rejected before reaching the database.
func TestSavingDraftRejectsMismatchedRevision(t *testing.T) {
	repo := &savingRepo{}
	problem := uuid.New()
	uc := &DraftUseCase{Repo: repo, Problems: singleProblem{uuid.New()}, Permissions: allowProblemPermissions{}}
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
	_, err := uc.SaveDraft(ctx, problem, 3, "key-1", model.Draft{ProblemID: problem, Revision: 4})
	require.Error(t, err)
	require.Empty(t, repo.saved)
}
