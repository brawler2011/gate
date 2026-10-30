package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type manifestFaultRepo struct {
	interfaces.ProblemsRepo
	manifestErr, limitsErr, titleErr, readErr error
	limits                                    [2]int
	title                                     string
}

func (r *manifestFaultRepo) GetProblemManifest(ctx context.Context, id uuid.UUID) ([]byte, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	return r.ProblemsRepo.GetProblemManifest(ctx, id)
}
func (r *manifestFaultRepo) UpdateProblemManifest(ctx context.Context, id uuid.UUID, data []byte) error {
	if r.manifestErr != nil {
		return r.manifestErr
	}
	return r.ProblemsRepo.UpdateProblemManifest(ctx, id, data)
}
func (r *manifestFaultRepo) UpdateProblemLimits(_ context.Context, _ uuid.UUID, time, memory int) error {
	if r.limitsErr != nil {
		return r.limitsErr
	}
	r.limits = [2]int{time, memory}
	return nil
}
func (r *manifestFaultRepo) UpdateProblem(_ context.Context, _ uuid.UUID, update *models.ProblemUpdate) error {
	if r.titleErr != nil {
		return r.titleErr
	}
	r.title = *update.Title
	return nil
}

type manifestFaultStorage struct {
	storage.Storage
	readPath, writePath, deletePath string
	fault               error
}

func (s *manifestFaultStorage) DeleteFile(ctx context.Context, bucket, key string) error {
	if s.deletePath != "" && strings.HasSuffix(key, s.deletePath) { return s.fault }
	return s.Storage.DeleteFile(ctx, bucket, key)
}

func (s *manifestFaultStorage) DownloadFile(ctx context.Context, bucket, key string, tag *string) (io.ReadCloser, string, error) {
	if s.readPath != "" && strings.HasSuffix(key, s.readPath) {
		return nil, "", s.fault
	}
	return s.Storage.DownloadFile(ctx, bucket, key, tag)
}
func (s *manifestFaultStorage) UploadFile(ctx context.Context, bucket, key string, data io.Reader, contentType string) error {
	if s.writePath != "" && strings.HasSuffix(key, s.writePath) {
		return s.fault
	}
	return s.Storage.UploadFile(ctx, bucket, key, data, contentType)
}

func TestWorkshopSaveManifest_UnreadableLegacyYAML(t *testing.T) {
	for _, kind := range []string{"malformed", "unavailable", "missing", "database"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			uc, id := newWorkshopValidatorTest(t, false, nil, nil)
			manifest, err := uc.GetManifest(ctx, id)
			require.NoError(t, err)
			oldDB, err := uc.problemsRepo.GetProblemManifest(ctx, id)
			require.NoError(t, err)
			store := uc.workspaceStorage.storage
			sentinel := errors.New("temporary outage")
			switch kind {
			case "malformed":
				require.NoError(t, uc.workspaceStorage.WriteFile(ctx, id, "problem.yaml", []byte("subtasks: [")))
			case "unavailable":
				uc.workspaceStorage.storage = &manifestFaultStorage{Storage: store, readPath: "problem.yaml", fault: sentinel}
			case "missing":
				require.NoError(t, uc.workspaceStorage.DeleteFile(ctx, id, "problem.yaml"))
			case "database":
				uc.problemsRepo = &manifestFaultRepo{ProblemsRepo: uc.problemsRepo, readErr: sentinel}
			}
			manifest.TimeLimitMs = 2345
			require.Error(t, uc.SaveManifest(ctx, id, manifest))
			if kind == "database" {
				uc.problemsRepo = uc.problemsRepo.(*manifestFaultRepo).ProblemsRepo
			}
			after, err := uc.problemsRepo.GetProblemManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, oldDB, after)
			uc.workspaceStorage.storage = store
			if kind == "malformed" {
				data, err := uc.ReadProblemFile(ctx, id, "problem.yaml")
				require.NoError(t, err)
				require.Equal(t, "subtasks: [", string(data))
			}
		})
	}
}

func TestWorkshopSaveManifest_FailuresAndRecovery(t *testing.T) {
	for _, kind := range []string{"manifest", "yaml-write", "statement-write", "limits", "title", "yaml-read"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			uc, id := newWorkshopValidatorTest(t, false, map[string]string{"problem.yaml": "solutions: {main.cpp: main}\nsubtasks:\n custom:\n  tests: [{manual: keep.in, sample: true}]\n"}, nil)
			manifest, err := uc.GetManifest(ctx, id)
			require.NoError(t, err)
			tests, err := uc.GetTestsConfig(ctx, id)
			require.NoError(t, err)
			before, err := uc.problemsRepo.GetProblemManifest(ctx, id)
			require.NoError(t, err)
			repo := &manifestFaultRepo{ProblemsRepo: uc.problemsRepo}
			uc.problemsRepo = repo
			store := &manifestFaultStorage{Storage: uc.workspaceStorage.storage, fault: errors.New("injected failure")}
			uc.workspaceStorage.storage = store
			switch kind {
			case "manifest":
				repo.manifestErr = store.fault
			case "yaml-write":
				store.writePath = "problem.yaml"
			case "statement-write":
				store.writePath = "statements/en.md"
			case "limits":
				repo.limitsErr = store.fault
			case "title":
				repo.titleErr = store.fault
			case "yaml-read":
				store.readPath = "problem.yaml"
			}
			manifest.TimeLimitMs = 2345
			manifest.MemoryLimitMb = 345
			manifest.Statement.Title = "Recovered title"
			require.ErrorIs(t, uc.SaveManifest(ctx, id, manifest), store.fault)
			// Failures before the authoritative commit leave its bytes untouched.
			if kind == "manifest" || kind == "yaml-read" {
				after, err := repo.GetProblemManifest(ctx, id)
				require.NoError(t, err)
				require.Equal(t, before, after)
			} else {
				// Repeated recovery cannot claim success while a projection still fails.
				_, err := uc.GetManifest(ctx, id)
				require.ErrorIs(t, err, store.fault)
			}
			repo.manifestErr = nil
			repo.limitsErr = nil
			repo.titleErr = nil
			store.writePath = ""
			store.readPath = ""
			// A fresh use case simulates recovery after the process has restarted.
			uc = NewWorkshopUseCase(repo, uc.workspaceStorage, nil, nil)
			if kind == "manifest" || kind == "yaml-read" {
				require.NoError(t, uc.SaveManifest(ctx, id, manifest))
			}
			got, err := uc.GetManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, 2345, got.TimeLimitMs)
			require.Equal(t, [2]int{2345, 345}, repo.limits)
			require.Equal(t, "Recovered title", repo.title)
			afterTests, err := uc.GetTestsConfig(ctx, id)
			require.NoError(t, err)
			require.Equal(t, tests, afterTests)
			config, err := uc.readProblemConfig(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "main", config.Solutions["main.cpp"])
			stmt, err := uc.ReadProblemFile(ctx, id, "statements/en.md")
			require.NoError(t, err)
			require.Contains(t, string(stmt), "Recovered title")
			require.NoError(t, uc.SaveManifest(ctx, id, got))
		})
	}
}

func TestWorkshopSaveManifest_RepairsCommittedYAMLWithoutDefaults(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{"problem.yaml": "subtasks: {special: {tests: [{manual: special.input}]}}"}, nil)
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.NoError(t, uc.SaveManifest(ctx, id, manifest))
	require.NoError(t, uc.workspaceStorage.WriteFile(ctx, id, "problem.yaml", []byte("subtasks: [")))
	_, err = uc.GetManifest(ctx, id)
	require.NoError(t, err)
	tests, err := uc.GetTestsConfig(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "special.input", tests.Tests[0].Filename)
	// A transient read failure does not overwrite a projection or report success.
	store := &manifestFaultStorage{Storage: uc.workspaceStorage.storage, readPath: "problem.yaml", fault: errors.New("unavailable")}
	uc.workspaceStorage.storage = store
	_, err = uc.GetManifest(ctx, id)
	require.ErrorIs(t, err, store.fault)
	store.readPath = ""
	_, err = uc.GetManifest(ctx, id)
	require.NoError(t, err)
}

func TestWorkshopStatementFileEdit_CommitsManifest(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, nil, nil)
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.NoError(t, uc.SaveManifest(ctx, id, manifest))
	manifest.Statement.Title = "Edited in file"
	manifest.Statement.Legend = "Keep this edit"
	require.NoError(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "statements/en.md", Content: []byte(RenderStatementMarkdown(manifest.Statement))}))
	got, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, manifest.Statement, got.Statement)
	require.ErrorContains(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "problem.yaml", Content: []byte("subtasks: {}")}), "cannot modify")
}

func TestWorkshopInit_RequiresAbsentConfiguration(t *testing.T) {
	for _, kind := range []string{"existing", "unavailable", "new"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			uc, id := newWorkshopValidatorTest(t, false, nil, nil)
			repo := uc.problemsRepo.(*problemImportMockProblemsRepo)
			delete(repo.manifests, id)
			before, err := uc.ReadProblemFile(ctx, id, "problem.yaml")
			require.NoError(t, err)
			switch kind {
			case "existing":
				require.ErrorContains(t, uc.InitProblemWorkshop(ctx, id, "New"), "existing problem.yaml")
				after, err := uc.ReadProblemFile(ctx, id, "problem.yaml")
				require.NoError(t, err)
				require.Equal(t, before, after)
			case "unavailable":
				fault := errors.New("storage read unavailable")
				uc.workspaceStorage.storage = &manifestFaultStorage{Storage: uc.workspaceStorage.storage, readPath: "problem.yaml", fault: fault}
				require.ErrorIs(t, uc.InitProblemWorkshop(ctx, id, "New"), fault)
			case "new":
				require.NoError(t, uc.workspaceStorage.DeleteFile(ctx, id, "problem.yaml"))
				require.NoError(t, uc.InitProblemWorkshop(ctx, id, "New"))
				manifest, err := uc.GetManifest(ctx, id)
				require.NoError(t, err)
				require.Equal(t, "New", manifest.Statement.Title)
				tests, err := uc.GetTestsConfig(ctx, id)
				require.NoError(t, err)
				require.NotEmpty(t, tests.Tests)
			}
		})
	}
}

func TestWorkshopTestsConfig_RecoversPartialUpdate(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, nil, nil)
	tests, err := uc.GetTestsConfig(ctx, id)
	require.NoError(t, err)
	tests.Tests[0].Filename = "replacement.in"
	fault := errors.New("YAML upload failed")
	store := &manifestFaultStorage{Storage: uc.workspaceStorage.storage, writePath: "problem.yaml", fault: fault}
	uc.workspaceStorage.storage = store
	require.ErrorIs(t, uc.UpdateTestsConfig(ctx, id, tests), fault)
	store.writePath = ""
	got, err := uc.GetTestsConfig(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "replacement.in", got.Tests[0].Filename)
	for _, path := range []string{"./problem.yaml", "statements/../problem.yaml"} {
		require.ErrorContains(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: path}), "cannot modify")
		require.ErrorContains(t, uc.DeleteProblemFile(ctx, id, path), "cannot delete")
	}
}

func TestWorkshopReadProblemFile_RecoversProjection(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, nil, nil)
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	store := &manifestFaultStorage{Storage: uc.workspaceStorage.storage, writePath: "problem.yaml", fault: errors.New("write failed")}
	uc.workspaceStorage.storage = store
	manifest.TimeLimitMs = 4321
	require.ErrorIs(t, uc.SaveManifest(ctx, id, manifest), store.fault)
	_, err = uc.ReadProblemFile(ctx, id, "problem.yaml")
	require.ErrorIs(t, err, store.fault)
	store.writePath = ""
	data, err := uc.ReadProblemFile(ctx, id, "problem.yaml")
	require.NoError(t, err)
	require.Contains(t, string(data), "time_ms: 4321")
	require.ErrorContains(t, uc.DeleteProblemFile(ctx, id, "statements/en.md"), "cannot delete derived metadata")
}
