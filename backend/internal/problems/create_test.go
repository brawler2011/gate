package problems

import (
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

type creationCatalog struct{ ProblemsRepo }

func (creationCatalog) GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error) {
	return problemsmodel.Problem{}, pkg.ErrNotFound
}

type creationStorage struct {
	storage.Storage
	active   *bool
	failure  error
	uploaded bool
}

func (s *creationStorage) UploadFile(_ context.Context, _, _ string, reader io.Reader, _ string) error {
	if *s.active {
		return errors.New("object I/O inside SQL transaction")
	}
	if s.failure != nil {
		return s.failure
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("read uploaded package: %w", err)
	}
	s.uploaded = true
	return nil
}

type creationTransactions struct {
	ProblemTxStore
	active, called bool
	object         *creationStorage
	owner          bool
	hash           string
	metadata       *problemsmodel.WorkspaceMetadata
}

func (p *creationTransactions) Within(_ context.Context, fn func(ProblemTxStore) error) error {
	if !p.object.uploaded {
		return errors.New("transaction opened before package upload")
	}
	p.active, p.called = true, true
	defer func() { p.active = false }()
	return fn(p)
}
func (*creationTransactions) CreateProblem(context.Context, *problemsmodel.CreateProblemParams) error {
	return nil
}
func (p *creationTransactions) CreateProblemMember(context.Context, *problemsmodel.CreateProblemMemberParams) error {
	p.owner = true
	return nil
}
func (p *creationTransactions) CommitWorkspace(_ context.Context, _ uuid.UUID, previous, hash string, metadata *problemsmodel.WorkspaceMetadata) error {
	if previous != "" {
		return errors.New("new problem has an existing reference")
	}
	p.hash, p.metadata = hash, metadata
	return nil
}

func TestProblemCreationUploadsBeforeAtomicCatalogWrites(t *testing.T) {
	for _, failure := range []error{nil, errors.New("object storage unavailable")} {
		t.Run(map[bool]string{true: "upload fails", false: "created"}[failure != nil], func(t *testing.T) {
			tx := &creationTransactions{}
			object := &creationStorage{active: &tx.active, failure: failure}
			tx.object = object
			workspace := NewWorkspaceStorage(object, "workspace", nil)
			uc := NewProblemsUseCase(creationCatalog{}, workspace, tx, nil, nil)
			user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			id, err := uc.CreateProblem(accessmodel.WithUser(problemActorContext(), user), &problemsmodel.CreateProblemInput{OrganizationID: uuid.New(), Title: "New task", TemplateID: "builtin:a-plus-b"})
			if failure != nil {
				require.ErrorIs(t, err, failure)
				require.False(t, tx.called)
				require.Equal(t, uuid.Nil, id)
				return
			}
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, id)
			require.True(t, tx.owner)
			require.Len(t, tx.hash, 64)
			require.NotNil(t, tx.metadata)
			require.Equal(t, "New task", tx.metadata.Title)
		})
	}
}
