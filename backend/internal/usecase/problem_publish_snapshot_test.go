package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type snapshotPackages struct {
	interfaces.PackagesRepo
	created, ready bool
}

func (r *snapshotPackages) CreatePackage(_ context.Context, p *models.CreatePackageParams) (models.ProblemPackage, error) {
	r.created = true
	return models.ProblemPackage{ID: p.ID, Version: 1}, nil
}
func (r *snapshotPackages) UpdatePackageStatus(_ context.Context, p *models.UpdatePackageStatusParams) error {
	r.ready = p.Status == "ready"
	return nil
}

type publicationStorage struct {
	storage.Storage
	entered, release chan struct{}
	archive          []byte
}

func (s *publicationStorage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read package upload: %w", err)
	}
	s.archive = data
	close(s.entered)
	<-s.release
	return s.Storage.UploadFile(ctx, bucket, key, bytes.NewReader(data), contentType)
}

type observedWorkspaceLocker struct {
	interfaces.WorkspaceLocker
	attempted chan struct{}
}

func (l *observedWorkspaceLocker) WithWorkspaceLock(ctx context.Context, id uuid.UUID, fn func(context.Context) error) error {
	l.attempted <- struct{}{}
	return l.WorkspaceLocker.WithWorkspaceLock(ctx, id, fn)
}

func TestPublishProblem_SerializesSaveAndRecoversSnapshot(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{
		"problem.yaml":       "format_version: '1.0'\ntitle: 'Original'\ntype: pass-fail\nchecker: checkers/check.cpp\nlimits: {time_ms: 1000, memory_mb: 256}\nsubtasks: {}\n",
		"checkers/check.cpp": "int main() {}",
	}, nil)
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	manifest.FilesMetadata = []models.FileMetadata{{Type: "checker", Filename: "checkers/check.cpp", Compiler: "cpp17"}}
	// Commit succeeds but projecting limits fails; publication must recover it.
	repo := &manifestFaultRepo{ProblemsRepo: uc.problemsRepo, limitsErr: errors.New("limits outage")}
	uc.problemsRepo = repo
	require.Error(t, uc.SaveManifest(ctx, id, manifest))
	repo.limitsErr = nil
	ws := uc.workspaceStorage
	locker := &observedWorkspaceLocker{WorkspaceLocker: ws.locker, attempted: make(chan struct{}, 4)}
	ws.locker = locker
	packages := &snapshotPackages{}
	store := &publicationStorage{Storage: storage.NewLocalStorage(t.TempDir()), entered: make(chan struct{}), release: make(chan struct{})}
	publisher := NewProblemPublishUseCase(repo, packages, ws, store, "published")
	published := make(chan error, 1)
	go func() { _, err := publisher.PublishProblem(ctx, id); published <- err }()
	<-locker.attempted
	select {
	case <-store.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("publication did not reach upload")
	}
	// Upload holds the operation lock even after all workspace files were copied.
	manifest.Statement.Title = "Concurrent update"
	manifest.TimeLimitMs = 9999
	saved := make(chan error, 1)
	go func() { saved <- uc.SaveManifest(ctx, id, manifest) }()
	<-locker.attempted
	select {
	case err := <-saved:
		t.Fatalf("save passed publication lock: %v", err)
	default:
	}
	close(store.release)
	require.NoError(t, <-published)
	require.NoError(t, <-saved)
	require.True(t, packages.created)
	require.True(t, packages.ready)
	archive, err := zip.NewReader(bytes.NewReader(store.archive), int64(len(store.archive)))
	require.NoError(t, err)
	files := map[string]string{}
	for _, file := range archive.File {
		r, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		files[file.Name] = string(data)
	}
	require.Contains(t, files["problem.yaml"], "time_ms: 1000")
	require.NotContains(t, files["statements/en.md"], "Concurrent update")
	got, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 9999, got.TimeLimitMs)
}

func TestPublishProblem_BlocksUnrecoveredSnapshot(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, nil, nil)
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.NoError(t, uc.SaveManifest(ctx, id, manifest))
	fault := errors.New("limits unavailable")
	repo := &manifestFaultRepo{ProblemsRepo: uc.problemsRepo, limitsErr: fault}
	packages := &snapshotPackages{}
	publisher := NewProblemPublishUseCase(repo, packages, uc.workspaceStorage, nil, "published")
	result, err := publisher.PublishProblem(ctx, id)
	require.ErrorIs(t, err, fault)
	require.Nil(t, result)
	require.False(t, packages.created)
}
