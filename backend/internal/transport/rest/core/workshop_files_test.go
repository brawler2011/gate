package core

import (
	"context"
	"encoding/json"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type workshopFilesRepo struct {
	interfaces.ProblemsRepo
	mu   sync.Mutex
	hash string
}

func (r *workshopFilesRepo) GetWorkspaceHash(context.Context, uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hash, nil
}
func (r *workshopFilesRepo) CommitWorkspace(_ context.Context, _ uuid.UUID, previousHash, hash string, _ *models.WorkspaceMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != previousHash {
		return pkg.ErrConflict
	}
	r.hash = hash
	return nil
}

type workshopFilesSecurity struct{}

func (workshopFilesSecurity) HandleCookieAuth(ctx context.Context, _ corev1.OperationName, _ corev1.CookieAuth) (context.Context, error) {
	return ctx, nil
}

func TestWorkshopNestedTestFiles_HTTP(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	repo := &workshopFilesRepo{}
	ws := usecase.NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "test", repo)
	for path, content := range map[string]string{
		"problem.yaml":         "subtasks:\n  samples:\n    tests:\n      - manual: nested/edge\n",
		"tests/nested/edge.in": "original input", "tests/nested/edge.ans": "original answer",
		"tests/nested/deeper/extra.in": "extra", "tests2/outside.in": "outside", "solutions/main.cpp": "solution",
	} {
		require.NoError(t, ws.WriteFile(ctx, id, path, []byte(content)))
	}
	handler := &CoreServer{workshopUC: usecase.NewWorkshopUseCase(ws, nil)}
	server, err := corev1.NewServer(handler, workshopFilesSecurity{}, corev1.WithErrorHandler(func(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
		w.WriteHeader(pkg.ToREST(err))
	}))
	require.NoError(t, err)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/problems/"+id.String()+"/tests"+path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "test"})
		req.Header.Set("Content-Type", "application/octet-stream")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, req)
		return response
	}
	response := request(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var listed struct {
		Files []struct {
			Path        string `json:"path"`
			IsDirectory bool   `json:"is_directory"`
			Size        int64  `json:"size"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &listed))
	var paths []string
	for _, file := range listed.Files {
		require.False(t, file.IsDirectory)
		require.Positive(t, file.Size)
		paths = append(paths, file.Path)
	}
	require.ElementsMatch(t, []string{"tests/nested/edge.in", "tests/nested/edge.ans", "tests/nested/deeper/extra.in"}, paths)
	// The generic workspace tree stays shallow; only the tests collection expands directories.
	tree, err := handler.workshopUC.ListProblemFiles(ctx, id, "tests")
	require.NoError(t, err)
	require.Len(t, tree, 1)
	require.True(t, tree[0].IsDirectory)
	for _, name := range []string{"nested/edge.in", "nested/edge.ans"} {
		response = request(http.MethodGet, "/"+url.PathEscape(name), "")
		require.Equal(t, http.StatusOK, response.Code)
		require.True(t, strings.HasPrefix(response.Body.String(), "original"))
		response = request(http.MethodPut, "/"+url.PathEscape(name), "edited "+name)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		response = request(http.MethodGet, "/"+url.PathEscape(name), "")
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "edited "+name, response.Body.String())
	}
	for _, name := range []string{"../problem.yaml", "nested/../../problem.yaml", "/problem.yaml", `nested\edge.in`, "nested//edge.in", "nested/./edge.in", "nested/../edge.in", "nested/edge\x00.in"} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			response = request(method, "/"+url.PathEscape(name), "unsafe")
			require.Equal(t, http.StatusBadRequest, response.Code, method+" "+name)
		}
	}
	response = request(http.MethodDelete, "/"+url.PathEscape("nested/deeper/extra.in"), "")
	require.Equal(t, http.StatusOK, response.Code)
	response = request(http.MethodGet, "/"+url.PathEscape("nested/deeper/extra.in"), "")
	require.Equal(t, http.StatusNotFound, response.Code)
	outside, err := ws.ReadFile(ctx, id, "tests2/outside.in")
	require.NoError(t, err)
	require.Equal(t, "outside", string(outside))
}

type competingWorkshopPatch struct {
	interfaces.WorkshopUC
	beforeWrite func()
}

func (w competingWorkshopPatch) SaveManifest(ctx context.Context, id uuid.UUID, manifest *models.ProblemManifest) error {
	w.beforeWrite()
	return w.WorkshopUC.SaveManifest(ctx, id, manifest)
}
func (w competingWorkshopPatch) UpdateProblemFile(ctx context.Context, request models.UpdateFileRequest) error {
	w.beforeWrite()
	return w.WorkshopUC.UpdateProblemFile(ctx, request)
}

func TestWorkshopPatchesRejectConcurrentChanges_HTTP409(t *testing.T) {
	for _, patch := range []string{"limits", "statement"} {
		t.Run(patch, func(t *testing.T) {
			ctx := context.Background()
			id := uuid.New()
			repo := &workshopFilesRepo{}
			ws := usecase.NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "test", repo)
			uc := usecase.NewWorkshopUseCase(ws, nil)
			require.NoError(t, uc.InitProblemWorkshop(ctx, id, "Original"))
			beforeWrite := func() {
				current, err := uc.GetManifest(ctx, id)
				require.NoError(t, err)
				current.Statement.Title = "Concurrent title"
				require.NoError(t, uc.SaveManifest(ctx, id, current))
			}
			handler := &CoreServer{workshopUC: competingWorkshopPatch{WorkshopUC: uc, beforeWrite: beforeWrite}}
			var err error
			if patch == "limits" {
				_, err = handler.UpdateProblemLimits(ctx, &corev1.UpdateProblemLimitsRequest{TimeLimitMs: corev1.NewOptInt(4321)}, corev1.UpdateProblemLimitsParams{ProblemId: id})
			} else {
				_, err = handler.UpdateProblemStatement(ctx, &corev1.UpdateProblemStatementRequest{Legend: corev1.NewOptString("Stale legend")}, corev1.UpdateProblemStatementParams{ProblemId: id})
			}
			require.ErrorIs(t, err, pkg.ErrConflict)
			require.Equal(t, http.StatusConflict, pkg.ToREST(err))
			actual, err := uc.GetManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "Concurrent title", actual.Statement.Title)
			require.Equal(t, 1000, actual.TimeLimitMs)
			require.NotEqual(t, "Stale legend", actual.Statement.Legend)
		})
	}
}
