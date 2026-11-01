package main

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// spaHandler serves files and lets the client router resolve page URLs.
func spaHandler(directory string) http.Handler {
	files := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		info, err := fs.Stat(os.DirFS(directory), name)
		if err == nil && !info.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			if strings.HasPrefix(name, "assets/") && path.Ext(name) != ".html" {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(name, "assets/") || path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, directory+"/index.html")
	})
}
