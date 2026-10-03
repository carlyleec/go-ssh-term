package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	const index = "<!doctype html><html>React entry</html>"
	files := fstest.MapFS{
		"index.html":       {Data: []byte(index)},
		"assets/app.js":    {Data: []byte("console.log('app')")},
		"assets/app.css":   {Data: []byte("body { color: black; }")},
		"robots.txt":       {Data: []byte("User-agent: *")},
		"api/example.json": {Data: []byte(`{"not":"an API"}`)},
	}
	for _, tc := range []struct {
		method, path string
		status       int
		body         string
	}{
		{"GET", "/", 200, index},
		{"GET", "/login", 200, index},
		{"GET", "/connections?tab=one", 200, index},
		{"GET", "/connections/", 200, index},
		{"GET", "/unknown/page", 200, index},
		{"GET", "/assets/app.js", 200, "console.log('app')"},
		{"GET", "/assets/app.css", 200, "body { color: black; }"},
		{"GET", "/robots.txt", 200, "User-agent: *"},
		{"HEAD", "/login", 200, ""},
		{"HEAD", "/assets/app.js", 200, ""},
		{"GET", "/api", 404, "404 page not found\n"},
		{"POST", "/api/missing", 404, "404 page not found\n"},
		{"GET", "/api/example.json", 404, "404 page not found\n"},
		{"GET", "/assets", 404, "404 page not found\n"},
		{"GET", "/assets/", 404, "404 page not found\n"},
		{"GET", "/assets/missing", 404, "404 page not found\n"},
		{"GET", "/missing.js", 404, "404 page not found\n"},
		{"GET", "/missing.js/", 404, "404 page not found\n"},
		{"GET", "/favicon.ico", 404, "404 page not found\n"},
		{"GET", "/../secret", 404, "404 page not found\n"},
		{"POST", "/login", 405, "method not allowed\n"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			Handler(files).ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, nil))
			if rr.Code != tc.status || rr.Body.String() != tc.body {
				t.Fatalf("got %d %q; want %d %q", rr.Code, rr.Body.String(), tc.status, tc.body)
			}
			if tc.body == index && rr.Header().Get("Cache-Control") != "no-cache" {
				t.Error("SPA entry must be revalidated")
			}
			if tc.status == 405 && rr.Header().Get("Allow") != "GET, HEAD" {
				t.Error("missing allowed methods")
			}
		})
	}
}

func TestMissingBuild(t *testing.T) {
	rr := httptest.NewRecorder()
	Handler(fstest.MapFS{}).ServeHTTP(rr, httptest.NewRequest("GET", "/login", nil))
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "make build") {
		t.Fatalf("expected actionable missing-build response, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestAPIRoutesTakePrecedence(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/", Handler(fstest.MapFS{}))
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest("GET", "/api/health", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("API handler lost precedence: %d", rr.Code)
	}
}
