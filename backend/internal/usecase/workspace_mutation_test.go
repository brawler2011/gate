package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceMutation_FailedImportBlocksPublicationUntilCompleteRetry(t *testing.T) {
	for _, renamed := range []bool{false, true} {
		for _, failure := range []string{"delete", "upload", "manifest", "limits", "complete"} {
			t.Run(fmt.Sprintf("renamed=%v/%s", renamed, failure), func(t *testing.T) {
				ctx := context.Background()
				oldYAML := "format_version: '1.0'\ntitle: Old\ntype: pass-fail\nlimits: {time_ms: 1000, memory_mb: 256}\nchecker: checkers/check.cpp\nsubtasks: {}\n"
				uc, id := newWorkshopValidatorTest(t, false, map[string]string{"problem.yaml": oldYAML, "checkers/check.cpp": "OLD CHECKER", "tests/removed.in": "old"}, nil)
				manifest, err := uc.GetManifest(ctx, id)
				require.NoError(t, err)
				manifest.FilesMetadata = []models.FileMetadata{{Type: "checker", Filename: "checkers/check.cpp", Compiler: "cpp17"}}
				require.NoError(t, uc.SaveManifest(ctx, id, manifest))
				fault := errors.New("injected import failure")
				repo := &manifestFaultRepo{ProblemsRepo: uc.problemsRepo}
				store := &manifestFaultStorage{Storage: uc.workspaceStorage.storage, fault: fault}
				uc.workspaceStorage.storage = store
				switch failure {
				case "delete":
					store.deletePath = "checkers/check.cpp"
				case "upload":
					store.writePath = "problem.yaml"
				case "manifest":
					repo.manifestErr = fault
				case "limits":
					repo.limitsErr = fault
				case "complete":
					store.deletePath = pendingMutationPath
				}
				checker := "checkers/check.cpp"
				if renamed {
					checker = "checkers/replaced.cpp"
				}
				packageBytes := createZipBytes(t, map[string]string{"problem.yaml": fmt.Sprintf("format_version: '1.0'\ntitle: New\ntype: pass-fail\nlimits: {time_ms: 9999, memory_mb: 256}\nchecker: %s\nsubtasks: {}\n", checker), checker: "NEW CHECKER"})
				importer := NewProblemImportUseCase(repo, uc.workspaceStorage)
				_, err = importer.ImportProblemPackage(ctx, bytes.NewReader(packageBytes), int64(len(packageBytes)), id)
				require.ErrorIs(t, err, fault)
				// Restart with new storage/use-case objects: the marker is persisted, not in memory.
				ws := NewWorkspaceStorage(store, uc.workspaceStorage.bucket, &WorkspaceMutex{})
				restarted := NewWorkshopUseCase(repo, ws, nil, nil)
				_, err = restarted.GetManifest(ctx, id)
				require.ErrorContains(t, err, "import")
				_, err = restarted.ReadProblemFile(ctx, id, checker)
				require.ErrorContains(t, err, "incomplete")
				_, err = restarted.ListProblemFiles(ctx, id, "")
				require.ErrorContains(t, err, "incomplete")
				packages := &snapshotPackages{}
				publisher := NewProblemPublishUseCase(repo, packages, ws, storage.NewLocalStorage(t.TempDir()), "published")
				_, err = publisher.PublishProblem(ctx, id)
				require.ErrorContains(t, err, "incomplete")
				require.False(t, packages.created)
				require.ErrorContains(t, restarted.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "tests/new.in", Content: []byte("new")}), "retry it first")
				repo.manifestErr = nil
				repo.limitsErr = nil
				store.writePath = ""
				store.deletePath = ""
				_, err = NewProblemImportUseCase(repo, ws).ImportProblemPackage(ctx, bytes.NewReader(packageBytes), int64(len(packageBytes)), id)
				require.NoError(t, err)
				got, err := restarted.GetManifest(ctx, id)
				require.NoError(t, err)
				require.Equal(t, 9999, got.TimeLimitMs)
				data, err := ws.ReadFile(ctx, id, checker)
				require.NoError(t, err)
				require.Equal(t, "NEW CHECKER", string(data))
				_, err = ws.ReadFile(ctx, id, "tests/removed.in")
				require.ErrorIs(t, err, storage.ErrNotFound)
				if renamed {
					_, err = ws.ReadFile(ctx, id, "checkers/check.cpp")
					require.ErrorIs(t, err, storage.ErrNotFound)
				}
				_, err = publisher.PublishProblem(ctx, id)
				require.NoError(t, err)
				require.True(t, packages.ready)
			})
		}
	}
}

func TestWorkspaceMutation_FileChangesRequireMatchingRetry(t *testing.T) {
	for _, operation := range []string{"update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			uc, id := newWorkshopValidatorTest(t, false, map[string]string{"checkers/check.cpp": "old"}, nil)
			manifest, err := uc.GetManifest(ctx, id)
			require.NoError(t, err)
			manifest.FilesMetadata = []models.FileMetadata{{Type: "checker", Filename: "checkers/check.cpp", Compiler: "cpp17"}}
			require.NoError(t, uc.SaveManifest(ctx, id, manifest))
			fault := errors.New("manifest update failure")
			repo := &manifestFaultRepo{ProblemsRepo: uc.problemsRepo, manifestErr: fault}
			uc.problemsRepo = repo
			run := func() error {
				if operation == "delete" {
					return uc.DeleteProblemFile(ctx, id, "checkers/check.cpp")
				}
				return uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "checkers/check.cpp", Content: []byte("new")})
			}
			require.ErrorIs(t, run(), fault)
			_, err = uc.GetManifest(ctx, id)
			require.ErrorContains(t, err, "incomplete")
			repo.manifestErr = nil
			require.NoError(t, run())
			_, err = uc.GetManifest(ctx, id)
			require.NoError(t, err)
			if operation == "delete" {
				_, err = uc.workspaceStorage.ReadFile(ctx, id, "checkers/check.cpp")
				require.ErrorIs(t, err, storage.ErrNotFound)
			} else {
				data, err := uc.workspaceStorage.ReadFile(ctx, id, "checkers/check.cpp")
				require.NoError(t, err)
				require.Equal(t, "new", string(data))
			}
		})
	}
}

func TestWorkspaceMutation_InterruptedCallbackRemainsBlocked(t *testing.T) {
	for _, interrupt := range []string{"cancel", "panic"} {
		t.Run(interrupt, func(t *testing.T) {
			ctx := context.Background()
			ws := NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "workspace", &WorkspaceMutex{})
			id := uuid.New()
			callback := func(ctx context.Context) error {
				require.NoError(t, ws.WriteFile(ctx, id, "checkers/check.cpp", []byte("partial")))
				if interrupt == "panic" {
					panic("interrupted mutation")
				}
				return context.Canceled
			}
			if interrupt == "panic" {
				require.Panics(t, func() { _ = ws.withMutation(ctx, id, "import", callback) })
			} else {
				require.ErrorIs(t, ws.withMutation(ctx, id, "import", callback), context.Canceled)
			}
			restarted := NewWorkspaceStorage(ws.storage, ws.bucket, &WorkspaceMutex{})
			require.ErrorContains(t, restarted.checkMutation(ctx, id), "import")
			require.NoError(t, restarted.withMutation(ctx, id, "import", func(ctx context.Context) error {
				return restarted.WriteFile(ctx, id, "checkers/check.cpp", []byte("complete"))
			}))
			require.NoError(t, restarted.checkMutation(ctx, id))
		})
	}
}

func TestWorkspaceMutation_ValidationAndIntentFailuresDoNotChangeFiles(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{"checkers/check.cpp": "original"}, nil)
	require.Error(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "problem.yaml"}))
	require.NoError(t, uc.workspaceStorage.checkMutation(ctx, id))
	fault := errors.New("intent unavailable")
	uc.workspaceStorage.storage = &manifestFaultStorage{Storage: uc.workspaceStorage.storage, writePath: pendingMutationPath, fault: fault}
	require.ErrorIs(t, uc.UpdateProblemFile(ctx, models.UpdateFileRequest{ProblemID: id, Path: "checkers/check.cpp", Content: []byte("changed")}), fault)
	data, err := uc.workspaceStorage.ReadFile(ctx, id, "checkers/check.cpp")
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
	require.NoError(t, uc.workspaceStorage.checkMutation(ctx, id))
}

func TestWorkspaceMutation_InitializationRetryRepairsCommittedState(t *testing.T) {
	ctx := context.Background()
	uc, id := newWorkshopValidatorTest(t, false, nil, nil)
	repo := &manifestFaultRepo{ProblemsRepo: uc.problemsRepo, limitsErr: errors.New("limits failed")}
	delete(uc.problemsRepo.(*problemImportMockProblemsRepo).manifests, id)
	require.NoError(t, uc.workspaceStorage.DeleteProblemWorkspace(ctx, id))
	uc.problemsRepo = repo
	require.Error(t, uc.InitProblemWorkshop(ctx, id, "Initialized"))
	_, err := uc.GetManifest(ctx, id)
	require.ErrorContains(t, err, "initialize")
	repo.limitsErr = nil
	require.NoError(t, uc.InitProblemWorkshop(ctx, id, "Initialized"))
	manifest, err := uc.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "Initialized", manifest.Statement.Title)
}
