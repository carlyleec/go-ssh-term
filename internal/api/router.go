package api

import (
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

// NewRouter preserves the HTTP method behavior of the gateway's original mux.
func NewRouter() *chi.Mux {
	mux := chi.NewRouter()
	mux.Use(canonicalPath)
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead && !mux.Match(chi.NewRouteContext(), http.MethodHead, r.URL.EscapedPath()) {
				// Route HEAD through GET without changing the method seen by handlers.
				chi.RouteContext(r.Context()).RouteMethod = http.MethodGet
			}
			next.ServeHTTP(w, r)
		})
	})
	mux.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		var allowed []string
		for _, method := range []string{"CONNECT", "DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT", "TRACE"} {
			matches := mux.Match(chi.NewRouteContext(), method, r.URL.EscapedPath())
			if method == http.MethodHead {
				matches = matches || mux.Match(chi.NewRouteContext(), http.MethodGet, r.URL.EscapedPath())
			}
			if matches {
				allowed = append(allowed, method)
			}
		}
		if len(allowed) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})
	return mux
}

// WithFrontend isolates API misses and method errors from the SPA fallback.
func WithFrontend(apiRoutes http.Handler, frontend http.Handler) http.Handler {
	root := chi.NewRouter()
	root.Use(canonicalPath)
	root.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		target := "/api/"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})
	root.Handle("/api/*", apiRoutes)
	root.Handle("/*", frontend)
	return root
}

func canonicalPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean escaped segments so an encoded slash or dot remains parameter data.
		original := r.URL.EscapedPath()
		cleaned := path.Clean(original)
		if strings.HasSuffix(original, "/") && cleaned != "/" {
			cleaned += "/"
		}
		if r.Method != http.MethodConnect && cleaned != original {
			if r.URL.RawQuery != "" {
				cleaned += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, cleaned, http.StatusTemporaryRedirect)
			return
		}
		next.ServeHTTP(w, r)
	})
}
