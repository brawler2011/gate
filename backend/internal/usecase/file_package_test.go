package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	pb "github.com/criyle/go-judge/pb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type packageFaultRepo struct {
	interfaces.WorkspaceRepo
	readErr, commitErr error
	commits            int
}

func (r *packageFaultRepo) GetWorkspaceHash(ctx context.Context, id uuid.UUID) (string, error) {
	if r.readErr != nil {
		return "", r.readErr
	}
	return r.WorkspaceRepo.GetWorkspaceHash(ctx, id)
}
func (r *packageFaultRepo) CommitWorkspace(ctx context.Context, id uuid.UUID, previous, hash string, metadata *models.WorkspaceMetadata) error {
	r.commits++
	if r.commitErr != nil {
		return r.commitErr
	}
	return r.WorkspaceRepo.CommitWorkspace(ctx, id, previous, hash, metadata)
}

type packageFaultStorage struct {
	storage.Storage
	uploadErr, readErr error
	uploads            int
	beforeUpload       func(context.Context, string) error
}

func (s *packageFaultStorage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, contentType string) error {
	s.uploads++
	if s.uploadErr != nil {
		return s.uploadErr
	}
	if s.beforeUpload != nil {
		if err := s.beforeUpload(ctx, bucket); err != nil {
			return err
		}
	}
	return s.Storage.UploadFile(ctx, bucket, key, reader, contentType)
}
func (s *packageFaultStorage) DownloadFile(ctx context.Context, bucket, key string, etag *string) (io.ReadCloser, string, error) {
	if s.readErr != nil {
		return nil, "", s.readErr
	}
	return s.Storage.DownloadFile(ctx, bucket, key, etag)
}

func TestFilePackageCommitFailures(t *testing.T) {
	for _, phase := range []string{"upload", "commit", "download", "reference"} {
		t.Run(phase, func(t *testing.T) {
			ctx := context.Background()
			base := newProblemImportMockProblemsRepo()
			repo := &packageFaultRepo{WorkspaceRepo: base}
			store := &packageFaultStorage{Storage: storage.NewLocalStorage(t.TempDir())}
			ws := NewWorkspaceStorage(store, "workspaces", repo)
			uc := NewWorkshopUseCase(ws, nil)
			id := uuid.New()
			require.NoError(t, uc.InitProblemWorkshop(ctx, id, "Original"))
			originalHash, err := base.GetWorkspaceHash(ctx, id)
			require.NoError(t, err)
			manifest, err := uc.GetManifest(ctx, id)
			require.NoError(t, err)
			manifest.Statement.Title = "Changed"
			manifest.Statement.Legend = "Changed statement"
			manifest.TimeLimitMs = 4321
			failure := errors.New("injected failure")
			switch phase {
			case "upload":
				store.uploadErr = failure
			case "commit":
				repo.commitErr = failure
			case "download":
				store.readErr = failure
			case "reference":
				repo.readErr = failure
			}
			require.ErrorIs(t, uc.SaveManifest(ctx, id, manifest), failure)
			hash, err := base.GetWorkspaceHash(ctx, id)
			require.NoError(t, err)
			require.Equal(t, originalHash, hash)
			fresh := NewWorkshopUseCase(NewWorkspaceStorage(store.Storage, "workspaces", base), nil)
			actual, err := fresh.GetManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "Original", actual.Statement.Title)
			require.Equal(t, 1000, actual.TimeLimitMs)
			require.NotEqual(t, manifest.Statement.Legend, actual.Statement.Legend)
			store.uploadErr, store.readErr, repo.commitErr, repo.readErr = nil, nil, nil, nil
			require.NoError(t, uc.SaveManifest(ctx, id, manifest))
			actual, err = fresh.GetManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, manifest.Statement, actual.Statement)
			require.Equal(t, 4321, actual.TimeLimitMs)
			uploads, commits := store.uploads, repo.commits
			_, err = uc.GetManifest(ctx, id)
			require.NoError(t, err)
			_, err = uc.GetTestsConfig(ctx, id)
			require.NoError(t, err)
			require.Equal(t, uploads, store.uploads, "reads never write packages")
			require.Equal(t, commits, repo.commits, "reads never repair database state")
		})
	}
}

func TestFilePackageCorruptConfigurationDoesNotResetTests(t *testing.T) {
	for _, content := range []string{"subtasks: [", ""} {
		t.Run(content, func(t *testing.T) {
			uc, id := newWorkshopValidatorTest(t, false, nil, nil)
			files := map[string]string{"tests/custom.in": "keep"}
			if content != "" {
				files["problem.yaml"] = content
			}
			seedWorkspaceFiles(t, uc.workspaceStorage, id, files)
			previous, err := uc.workspaceStorage.repo.GetWorkspaceHash(context.Background(), id)
			require.NoError(t, err)
			require.Error(t, uc.SaveManifest(context.Background(), id, defaultManifest("Must not reset")))
			hash, err := uc.workspaceStorage.repo.GetWorkspaceHash(context.Background(), id)
			require.NoError(t, err)
			require.Equal(t, previous, hash)
			data, err := uc.ReadProblemFile(context.Background(), id, "tests/custom.in")
			require.NoError(t, err)
			require.Equal(t, "keep", string(data))
		})
	}
}

func TestFilePackageConcurrentCommits(t *testing.T) {
	ctx := context.Background()
	repo := newProblemImportMockProblemsRepo()
	store := storage.NewLocalStorage(t.TempDir())
	first := NewWorkspaceStorage(store, "workspaces", repo)
	second := NewWorkspaceStorage(store, "workspaces", repo)
	id := uuid.New()
	require.NoError(t, NewWorkshopUseCase(first, nil).InitProblemWorkshop(ctx, id, "Concurrent"))
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for i, ws := range []*WorkspaceStorage{first, second} {
		go func() {
			results <- ws.withPackage(ctx, id, func(ctx context.Context, _ *workspacePackage) error {
				entered <- struct{}{}
				<-release
				return ws.WriteFile(ctx, id, []string{"tests/a.in", "tests/b.in"}[i], []byte("new"))
			})
		}()
	}
	<-entered
	<-entered
	close(release)
	a, b := <-results, <-results
	require.True(t, a == nil && errors.Is(b, pkg.ErrConflict) || b == nil && errors.Is(a, pkg.ErrConflict))
	files, err := first.ListAllFiles(ctx, id)
	require.NoError(t, err)
	require.NotEqual(t, slices.Contains(files, "tests/a.in"), slices.Contains(files, "tests/b.in"))
}

type filePackageRecords struct{ interfaces.PackagesRepo }

func (filePackageRecords) CreatePackage(_ context.Context, params *models.CreatePackageParams) (models.ProblemPackage, error) {
	return models.ProblemPackage{ID: params.ID, Version: 1}, nil
}
func (filePackageRecords) UpdatePackageStatus(context.Context, *models.UpdatePackageStatusParams) error {
	return nil
}

func TestFilePackagePublicationUsesCapturedRevision(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{
		"problem.yaml":       "title: Original\nchecker: checkers/check.cpp\nsubtasks: {}\n",
		"checkers/check.cpp": "original checker",
	}, nil)
	repo := uc.workspaceStorage.repo.(*problemImportMockProblemsRepo)
	raw := uc.workspaceStorage.storage
	store := &packageFaultStorage{Storage: raw}
	store.beforeUpload = func(ctx context.Context, bucket string) error {
		if bucket != "packages" {
			return nil
		}
		// Publication has already captured its package. An independent writer can
		// commit a replacement without blocking publication or changing its files.
		other := NewWorkspaceStorage(raw, uc.workspaceStorage.bucket, repo)
		return other.withPackage(ctx, id, func(ctx context.Context, _ *workspacePackage) error {
			editor := NewWorkshopUseCase(other, nil)
			config, err := editor.readProblemConfig(ctx, id)
			if err != nil {
				return err
			}
			config.Title = "Replacement"
			if err := editor.writeProblemConfig(ctx, id, config); err != nil {
				return err
			}
			return other.WriteFile(ctx, id, "checkers/check.cpp", []byte("replacement checker"))
		})
	}
	publisher := NewProblemPublishUseCase(repo, filePackageRecords{}, uc.workspaceStorage, store, "packages")
	result, err := publisher.PublishProblem(ctx, id)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Version)
	keys, err := raw.ListFiles(ctx, "packages", "")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	reader, _, err := raw.DownloadFile(ctx, "packages", keys[0], nil)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	for _, file := range archive.File {
		content, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(content)
		require.NoError(t, err)
		require.NoError(t, content.Close())
		if file.Name == "problem.yaml" {
			require.Contains(t, string(data), "Original")
			require.NotContains(t, string(data), "Replacement")
		}
		if file.Name == "checkers/check.cpp" {
			require.Equal(t, "original checker", string(data))
		}
	}
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "Replacement", manifest.Statement.Title)
}

func TestFilePackageValidationDoesNotLockEditors(t *testing.T) {
	uc, id := newWorkshopValidatorTest(t, true, map[string]string{"validators/validator.cpp": "validator"}, nil)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	uc.sandbox = newWorkshopValidatorSandbox(t, func(_ context.Context, request *pb.Request) (*pb.Response, error) {
		once.Do(func() { close(entered); <-release })
		require.Equal(t, "invalid input\n", string(request.GetCmd()[0].GetFiles()[0].GetMemory().GetContent()))
		return pb.Response_builder{Results: []*pb.Response_Result{pb.Response_Result_builder{Status: pb.Response_Result_Accepted}.Build()}}.Build(), nil
	})
	done := make(chan error, 1)
	go func() { _, err := uc.ValidateAllTests(context.Background(), id, uuid.Nil); done <- err }()
	<-entered
	err := uc.UpdateProblemFile(context.Background(), models.UpdateFileRequest{ProblemID: id, Path: "tests/01.in", Content: []byte("replacement")})
	close(release)
	require.NoError(t, err)
	require.NoError(t, <-done)
	data, err := uc.ReadProblemFile(context.Background(), id, "tests/01.in")
	require.NoError(t, err)
	require.Equal(t, "replacement", string(data))
}

func TestFilePackageInitializationAndImportAreAtomic(t *testing.T) {
	ctx := context.Background()
	repo := newProblemImportMockProblemsRepo()
	store := &packageFaultStorage{Storage: storage.NewLocalStorage(t.TempDir())}
	ws := NewWorkspaceStorage(store, "workspaces", repo)
	uc := NewWorkshopUseCase(ws, nil)
	id := uuid.New()
	failure := errors.New("upload failed")
	store.uploadErr = failure
	require.ErrorIs(t, uc.InitProblemWorkshop(ctx, id, "Original"), failure)
	hash, err := repo.GetWorkspaceHash(ctx, id)
	require.NoError(t, err)
	require.Empty(t, hash)
	store.uploadErr = nil
	require.NoError(t, uc.InitProblemWorkshop(ctx, id, "Original"))
	require.NoError(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "checkers/obsolete.cpp", Content: []byte("obsolete")}))
	original, err := repo.GetWorkspaceHash(ctx, id)
	require.NoError(t, err)
	incoming := createZipBytes(t, map[string]string{
		"problem.yaml":     "format_version: '1.0'\ntitle: Imported\ntype: pass-fail\nchecker: checkers/new.cpp\nlimits: {time_ms: 2345, memory_mb: 512}\nsubtasks: {}\n",
		"checkers/new.cpp": "new checker",
	})
	importer := NewProblemImportUseCase(repo, ws)
	for _, phase := range []string{"upload", "commit"} {
		fault := &packageFaultRepo{WorkspaceRepo: repo}
		ws.repo = fault
		if phase == "upload" {
			store.uploadErr = failure
		} else {
			fault.commitErr = failure
		}
		_, err := importer.ImportProblemPackage(ctx, bytes.NewReader(incoming), int64(len(incoming)), id)
		require.ErrorIs(t, err, failure)
		hash, err = repo.GetWorkspaceHash(ctx, id)
		require.NoError(t, err)
		require.Equal(t, original, hash)
		content, err := uc.ReadProblemFile(ctx, id, "checkers/obsolete.cpp")
		require.NoError(t, err)
		require.Equal(t, "obsolete", string(content))
		store.uploadErr = nil
	}
	ws.repo = repo
	manifest, err := importer.ImportProblemPackage(ctx, bytes.NewReader(incoming), int64(len(incoming)), id)
	require.NoError(t, err)
	require.Equal(t, "Imported", manifest.Statement.Title)
	_, err = uc.ReadProblemFile(ctx, id, "checkers/obsolete.cpp")
	require.ErrorIs(t, err, storage.ErrNotFound)
	content, err := uc.ReadProblemFile(ctx, id, "checkers/new.cpp")
	require.NoError(t, err)
	require.Equal(t, "new checker", string(content))
	// A missing referenced archive must fail ordinary reads, but a full import
	// can still replace it without a marker, repair API or legacy adoption.
	hash, err = repo.GetWorkspaceHash(ctx, id)
	require.NoError(t, err)
	require.NoError(t, store.DeleteFile(ctx, ws.bucket, ws.getWorkspaceKey(id, hash)))
	_, err = uc.GetManifest(ctx, id)
	require.ErrorIs(t, err, storage.ErrNotFound)
	_, err = importer.ImportProblemPackage(ctx, bytes.NewReader(incoming), int64(len(incoming)), id)
	require.NoError(t, err)
	_, err = uc.GetManifest(ctx, id)
	require.NoError(t, err)
}

func TestFilePackageMetadataRoundTrip(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{"checkers/check.cpp": "checker"}, nil)
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	score := 250
	manifest.ProblemType = "scoring"
	manifest.MaxScore = &score
	manifest.FilesMetadata[0].Compiler = "cpp20"
	manifest.FilesMetadata[0].Dependencies = []models.Dependency{{Filename: "custom.h", Version: "1.0"}}
	require.NoError(t, uc.SaveManifest(ctx, id, manifest))
	actual, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, manifest.MaxScore, actual.MaxScore)
	require.Equal(t, manifest.ProblemType, actual.ProblemType)
	require.Equal(t, manifest.FilesMetadata, actual.FilesMetadata)
	require.False(t, actual.LastUpdated.IsZero())
	require.NoError(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "statements/en.md", Content: []byte("<!-- title -->\nEdited title\n\n<!-- legend -->\nEdited legend\n")}))
	actual, err = uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "Edited title", actual.Statement.Title)
	require.Equal(t, "Edited legend", actual.Statement.Legend)
	raw, err := uc.ReadProblemFile(ctx, id, "statements/en.md")
	require.NoError(t, err)
	require.NotContains(t, string(raw), "<!-- title -->")
}

func TestFilePackageRejectsStaleMetadataAndStatementPatches(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, nil, nil)
	first, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	stale, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	first.Statement.Title = "Concurrent title"
	require.NoError(t, uc.SaveManifest(ctx, id, first))
	stale.TimeLimitMs = 4321
	require.ErrorIs(t, uc.SaveManifest(ctx, id, stale), pkg.ErrConflict)
	require.ErrorIs(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Revision: stale.Revision, Path: "statements/en.md", Content: []byte("<!-- legend -->\nStale statement")}), pkg.ErrConflict)
	actual, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "Concurrent title", actual.Statement.Title)
	require.Equal(t, 1000, actual.TimeLimitMs)
	require.NotEqual(t, "Stale statement", actual.Statement.Legend)
	require.Equal(t, first.Revision, actual.Revision)
}

func TestFilePackagePreservesSolutionBuildSettings(t *testing.T) {
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{
		"problem.yaml":       "title: Settings\ntype: pass-fail\nlimits: {time_ms: 1000, memory_mb: 256}\nsubtasks: {}\nsolutions: {main.cpp: main}\nsources:\n solutions/main.cpp: {compiler: cpp20, dependencies: [../lib/helper.h]}\n",
		"solutions/main.cpp": "solution", "lib/helper.h": "header",
	}, nil)
	ctx := context.Background()
	before, err := uc.readProblemConfig(ctx, id)
	require.NoError(t, err)
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	manifest.TimeLimitMs = 2345
	require.NoError(t, uc.SaveManifest(ctx, id, manifest))
	after, err := uc.readProblemConfig(ctx, id)
	require.NoError(t, err)
	require.Equal(t, before.Sources["solutions/main.cpp"], after.Sources["solutions/main.cpp"])
	require.Equal(t, before.Solutions, after.Solutions)
}

func TestFilePackageRejectsNullAndEmptyYAML(t *testing.T) {
	for _, content := range []string{"null\n", "", "[]\n", "subtasks: ["} {
		t.Run(content, func(t *testing.T) {
			uc, id := newWorkshopValidatorTest(t, false, nil, nil)
			seedWorkspaceFiles(t, uc.workspaceStorage, id, map[string]string{"problem.yaml": content, "tests/custom.in": "keep"})
			ctx := context.Background()
			previous, err := uc.workspaceStorage.repo.GetWorkspaceHash(ctx, id)
			require.NoError(t, err)
			_, err = uc.GetManifest(ctx, id)
			require.Error(t, err)
			initialized, err := uc.IsInitialized(ctx, id)
			require.Error(t, err)
			require.False(t, initialized)
			require.Error(t, uc.SaveManifest(ctx, id, defaultManifest("Must not overwrite")))
			current, err := uc.workspaceStorage.repo.GetWorkspaceHash(ctx, id)
			require.NoError(t, err)
			require.Equal(t, previous, current)
		})
	}
}

func TestFilePackageRejectsInvalidImportBeforeConversion(t *testing.T) {
	for _, content := range []string{"null\n", "", "[]\n", "subtasks: ["} {
		t.Run(content, func(t *testing.T) {
			ctx := context.Background()
			uc, id := newWorkshopValidatorTest(t, false, map[string]string{"tests/custom.in": "keep"}, nil)
			previous, err := uc.workspaceStorage.repo.GetWorkspaceHash(ctx, id)
			require.NoError(t, err)
			data := createZipBytes(t, map[string]string{"problem.yaml": content})
			importer := NewProblemImportUseCase(uc.workspaceStorage.repo.(*problemImportMockProblemsRepo), uc.workspaceStorage)
			_, err = importer.ImportProblemPackage(ctx, bytes.NewReader(data), int64(len(data)), id)
			require.Error(t, err)
			current, err := uc.workspaceStorage.repo.GetWorkspaceHash(ctx, id)
			require.NoError(t, err)
			require.Equal(t, previous, current)
			fresh := NewWorkshopUseCase(NewWorkspaceStorage(uc.workspaceStorage.storage, uc.workspaceStorage.bucket, uc.workspaceStorage.repo), nil)
			original, err := fresh.GetManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "Validator test", original.Statement.Title)
			actual, err := fresh.ReadProblemFile(ctx, id, "tests/custom.in")
			require.NoError(t, err)
			require.Equal(t, "keep", string(actual))
		})
	}
}
