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

func TestSPAHandlerCacheControl(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "assets"), 0o700))
	for _, name := range []string{
		"index.html", "logo.svg", "assets/app-Abc12345.js", "assets/app-Abc12345.css",
		"assets/logo-Abc12345.svg", "assets/font-Abc12345.woff2", "assets/page.html",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("content"), 0o600))
	}
	handler := spaHandler(dir)
	for _, tt := range []struct {
		url    string
		status int
		cache  string
	}{
		{"/", http.StatusOK, "no-cache"},
		{"/settings", http.StatusOK, "no-cache"},
		{"/index.html", http.StatusMovedPermanently, "no-cache"},
		{"/logo.svg", http.StatusOK, "no-cache"},
		{"/assets/page.html", http.StatusOK, "no-cache"},
		{"/assets/app-Abc12345.js", http.StatusOK, "public, max-age=31536000, immutable"},
		{"/assets/app-Abc12345.css", http.StatusOK, "public, max-age=31536000, immutable"},
		{"/assets/logo-Abc12345.svg", http.StatusOK, "public, max-age=31536000, immutable"},
		{"/assets/font-Abc12345.woff2", http.StatusOK, "public, max-age=31536000, immutable"},
		{"/assets/missing.js", http.StatusNotFound, ""},
	} {
		t.Run(tt.url, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				res := httptest.NewRecorder()
				handler.ServeHTTP(res, httptest.NewRequest(method, tt.url, nil))
				require.Equal(t, tt.status, res.Code)
				require.Equal(t, tt.cache, res.Header().Get("Cache-Control"))
				if method == http.MethodHead && tt.status == http.StatusOK {
					require.Empty(t, res.Body.String())
				}
			}
		})
	}
	for _, url := range []string{"/", "/logo.svg", "/assets/app-Abc12345.js"} {
		t.Run("revalidate"+url, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, url, nil))
			modified := res.Header().Get("Last-Modified")
			require.NotEmpty(t, modified)
			cache := res.Header().Get("Cache-Control")
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("If-Modified-Since", modified)
			res = httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			require.Equal(t, http.StatusNotModified, res.Code)
			require.Equal(t, cache, res.Header().Get("Cache-Control"))
			require.Empty(t, res.Body.String())
		})
	}
}
