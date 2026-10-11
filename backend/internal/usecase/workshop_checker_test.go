package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkshopTestSolution_CheckerPreparationErrors(t *testing.T) {
	storageErr := errors.New("storage unavailable")
	for _, tc := range []struct {
		name      string
		files     map[string]string
		deps      []models.Dependency
		readError error
		listError error
		wantError string
		wantCause error
	}{
		{name: "missing source", wantError: "failed to read checker checkers/checker.cpp", wantCause: storage.ErrNotFound},
		{name: "missing dependency", files: map[string]string{"checkers/checker.cpp": "checker"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to read checker dependency lib/testlib.h", wantCause: storage.ErrNotFound},
		{name: "dependency storage failure", files: map[string]string{"checkers/checker.cpp": "checker", "lib/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, readError: storageErr, wantError: "failed to read checker dependency checkers/testlib.h", wantCause: storageErr},
		{name: "compilation failure", files: map[string]string{"checkers/checker.cpp": "checker"}, wantError: "failed to compile checker checkers/checker.cpp: unsupported language: cpp"},
		{name: "relative dependency", files: map[string]string{"checkers/checker.cpp": "checker", "checkers/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to compile checker"},
		{name: "shared dependency", files: map[string]string{"checkers/checker.cpp": "checker", "lib/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to compile checker"},
		{name: "nested shared dependency", files: map[string]string{"checkers/checker.cpp": "checker", "lib/support/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to compile checker"},
		{name: "dependency listing failure", files: map[string]string{"checkers/checker.cpp": "checker", "lib/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, listError: storageErr, wantError: "failed to list checker dependencies", wantCause: storageErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &workshopCheckerStorage{Storage: storage.NewLocalStorage(t.TempDir()), readError: tc.readError, listError: tc.listError}
			uc, req := newWorkshopCheckerTest(t, store, tc.files, tc.deps)
			// An empty language config fails compilation without dispatching any execution.
			uc.sandbox = sandbox.NewSandbox(nil, &sandbox.Config{})
			report, err := uc.TestSolution(context.Background(), req)
			require.ErrorContains(t, err, tc.wantError)
			require.Nil(t, report)
			if tc.wantCause != nil {
				require.ErrorIs(t, err, tc.wantCause)
			}
			if tc.readError != nil {
				require.NotContains(t, store.readPaths, uc.workspaceStorage.getWorkspaceKey(req.ProblemID, "lib/testlib.h"))
			}
		})
	}
}

func TestWorkshopTestSolution_CheckerCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		code codes.Code
	}{
		{name: "cancelled", code: codes.Canceled},
		{name: "deadline", code: codes.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, req := newWorkshopCheckerTest(t, storage.NewLocalStorage(t.TempDir()), map[string]string{"checkers/checker.cpp": "checker"}, nil)
			client, err := sandbox.NewClient("127.0.0.1:1")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			cfg, err := sandbox.LoadConfig("../../languages.yaml")
			require.NoError(t, err)
			uc.sandbox = sandbox.NewSandbox(client, cfg)
			ctx, cancel := context.WithCancel(context.Background())
			if tc.code == codes.DeadlineExceeded {
				cancel()
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			}
			cancel()
			report, err := uc.TestSolution(ctx, req)
			require.ErrorContains(t, err, "failed to compile checker checkers/checker.cpp")
			require.Equal(t, tc.code, status.Code(err))
			require.Nil(t, report)
		})
	}
}

func TestWorkshopTestSolution_RequiresChecker(t *testing.T) {
	uc, req := newWorkshopCheckerTest(t, storage.NewLocalStorage(t.TempDir()), nil, nil)
	data, err := json.Marshal(defaultManifest("No checker"))
	require.NoError(t, err)
	require.NoError(t, uc.problemsRepo.UpdateProblemManifest(context.Background(), req.ProblemID, data))
	// A nil sandbox would panic if solution compilation were attempted.
	report, err := uc.TestSolution(context.Background(), req)
	require.ErrorContains(t, err, "problem must have a checker")
	require.Nil(t, report)
}

func newWorkshopCheckerTest(t *testing.T, store storage.Storage, files map[string]string, deps []models.Dependency) (*WorkshopUseCase, models.TestSolutionRequest) {
	t.Helper()
	problemID := uuid.New()
	repo := newProblemImportMockProblemsRepo()
	manifest := defaultManifest("Checker test")
	manifest.FilesMetadata = []models.FileMetadata{{Type: "checker", Filename: "checkers/checker.cpp", Dependencies: deps}}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	repo.manifests[problemID] = data
	ws := NewWorkspaceStorage(store, "test-workshop")
	require.NoError(t, ws.WriteFile(context.Background(), problemID, "solutions/main.cpp", []byte("solution")))
	require.NoError(t, ws.WriteFile(context.Background(), problemID, "problem.yaml", []byte("subtasks:\n  samples:\n    tests:\n      - manual: 01.in\n")))
	for path, content := range files {
		require.NoError(t, ws.WriteFile(context.Background(), problemID, path, []byte(content)))
	}
	return NewWorkshopUseCase(repo, ws, nil, nil), models.TestSolutionRequest{ProblemID: problemID, SolutionPath: "solutions/main.cpp"}
}

type workshopCheckerStorage struct {
	storage.Storage
	readError error
	listError error
	readPaths []string
}

func (s *workshopCheckerStorage) ListObjects(ctx context.Context, bucket, prefix string) ([]storage.ObjectInfo, error) {
	if s.listError != nil {
		return nil, s.listError
	}
	return s.Storage.ListObjects(ctx, bucket, prefix)
}

func (s *workshopCheckerStorage) DownloadFile(ctx context.Context, bucket, key string, etag *string) (io.ReadCloser, string, error) {
	s.readPaths = append(s.readPaths, key)
	if s.readError != nil && strings.HasSuffix(key, "checkers/testlib.h") {
		return nil, "", s.readError
	}
	return s.Storage.DownloadFile(ctx, bucket, key, etag)
}
