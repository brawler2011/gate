package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSPAHandler(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<div id=\"root\"></div>"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "assets"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("export {};"), 0o600))
	handler := spaHandler(dir)
	for _, url := range []string{"/", "/@alice", "/school/contest/A", "/submissions/123", "/admin/users"} {
		t.Run(url, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, url, nil))
			require.Equal(t, http.StatusOK, res.Code)
			require.Contains(t, res.Body.String(), "id=\"root\"")
			require.Equal(t, "no-cache", res.Header().Get("Cache-Control"))
		})
	}
	for _, url := range []string{"/assets/missing.js", "/missing.svg"} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, url, nil))
		require.Equal(t, http.StatusNotFound, res.Code)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	require.Equal(t, "export {};", res.Body.String())
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/school/contest/A", nil))
	require.Equal(t, http.StatusMethodNotAllowed, res.Code)
}
