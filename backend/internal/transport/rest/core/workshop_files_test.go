package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type workshopFilesRepo struct {
	ProblemsRepo
	mu   sync.Mutex
	hash string
}

func (r *workshopFilesRepo) GetWorkspaceHash(context.Context, uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hash, nil
}

func (r *workshopFilesRepo) CommitWorkspace(_ context.Context, _ uuid.UUID, previousHash, hash string, _ *problemsmodel.WorkspaceMetadata) error {
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
	return workshopFileContext(), nil
}

func TestWorkshopNestedTestFiles_HTTP(t *testing.T) {
	ctx := workshopFileContext()
	id := uuid.New()
	repo := &workshopFilesRepo{}
	ws := problems.NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "test", repo)
	for path, content := range map[string]string{
		"problem.yaml":         "subtasks:\n  samples:\n    tests:\n      - manual: nested/edge\n",
		"tests/nested/edge.in": "original input", "tests/nested/edge.ans": "original answer",
		"tests/nested/deeper/extra.in": "extra", "tests2/outside.in": "outside", "solutions/main.cpp": "solution",
	} {
		require.NoError(t, ws.WriteFile(ctx, id, path, []byte(content)))
	}
	handler := &CoreServer{problems: testProblemsModule{WorkshopUC: problems.NewWorkshopUseCase(ws, nil, workshopFilePermissions{})}}
	server, err := corev1.NewServer(handler, workshopFilesSecurity{}, corev1.WithErrorHandler(func(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
		w.WriteHeader(pkg.ToREST(err))
	}))
	require.NoError(t, err)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/problems/"+id.String()+"/tests"+path, strings.NewReader(body)).WithContext(ctx)
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
	tree, err := handler.problems.ListProblemFiles(ctx, id, "tests")
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

type conflictingWorkshop struct{ WorkshopUC }

func (conflictingWorkshop) UpdateLimits(context.Context, uuid.UUID, problems.LimitsPatch) (*problemsmodel.ProblemManifest, error) {
	return nil, pkg.ErrConflict
}
func (conflictingWorkshop) UpdateStatement(context.Context, uuid.UUID, string, problems.StatementPatch) (*problemsmodel.LocalizedStatement, error) {
	return nil, pkg.ErrConflict
}
func TestWorkshopPatchesReturnHTTP409(t *testing.T) {
	handler := &CoreServer{problems: testProblemsModule{WorkshopUC: conflictingWorkshop{}}}
	id := uuid.New()
	_, err := handler.UpdateProblemLimits(workshopFileContext(), &corev1.UpdateProblemLimitsRequest{TimeLimitMs: corev1.NewOptInt(4321)}, corev1.UpdateProblemLimitsParams{ProblemId: id})
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.Equal(t, http.StatusConflict, pkg.ToREST(err))
	_, err = handler.UpdateProblemStatement(workshopFileContext(), &corev1.UpdateProblemStatementRequest{Legend: corev1.NewOptString("Legend")}, corev1.UpdateProblemStatementParams{ProblemId: id})
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.Equal(t, http.StatusConflict, pkg.ToREST(err))
}

type workshopFilePermissions struct{}

func (workshopFilePermissions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return true, nil
}
func workshopFileContext() context.Context {
	return accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
}
