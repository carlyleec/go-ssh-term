package web

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// Handler serves built frontend files and falls back to the SPA for page URLs.
func Handler(files fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		if urlPath == "/api" || strings.HasPrefix(urlPath, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimSuffix(strings.TrimPrefix(urlPath, "/"), "/")
		if name != "" {
			if !fs.ValidPath(name) {
				http.NotFound(w, r)
				return
			}
			info, err := fs.Stat(files, name)
			if err == nil {
				if info.IsDir() {
					http.NotFound(w, r)
					return
				}
				http.ServeFileFS(w, r, files, name)
				return
			}
			if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "frontend file unavailable", http.StatusInternalServerError)
				return
			}
			if !isPagePath(urlPath) {
				http.NotFound(w, r)
				return
			}
		}

		index, err := fs.ReadFile(files, "index.html")
		if err != nil {
			http.Error(w, "frontend build unavailable; run make build", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
	})
}

func isPagePath(urlPath string) bool {
	urlPath = strings.TrimSuffix(urlPath, "/")
	return urlPath != "/assets" && !strings.HasPrefix(urlPath, "/assets/") && path.Ext(urlPath) == ""
}
