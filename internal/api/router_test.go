package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/carlyleec/go-ssh-term/internal/web"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// Compare observable routing behavior against the former ServeMux wiring.
func TestRouterParity(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("app()")},
	}
	oldAPI := http.NewServeMux()
	oldRoot := http.NewServeMux()
	oldRoot.Handle("/api/", oldAPI)
	oldRoot.Handle("/", web.Handler(files))
	newAPI := NewRouter()
	newRoot := WithFrontend(newAPI, web.Handler(files))
	oldConfig := huma.DefaultConfig("test", "1")
	oldConfig.CreateHooks = nil
	oldContract := humago.New(oldAPI, oldConfig)
	newContract := New(newAPI)
	type input struct {
		ID string `path:"id"`
	}
	type output struct {
		Body struct {
			ID string `json:"id"`
		}
	}
	for _, contract := range []huma.API{oldContract, newContract} {
		huma.Register(contract, huma.Operation{OperationID: "getItem", Method: "GET", Path: "/api/items/{id}"}, func(_ context.Context, in *input) (*output, error) {
			out := &output{}
			out.Body.ID = in.ID
			return out, nil
		})
		huma.Register(contract, huma.Operation{OperationID: "deleteItem", Method: "DELETE", Path: "/api/items/{id}"}, func(_ context.Context, _ *input) (*struct{}, error) { return nil, nil })
	}
	ready := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Method", r.Method)
		w.WriteHeader(http.StatusNoContent)
	})
	oldAPI.Handle("GET /api/readyz", ready)
	newAPI.Method("GET", "/api/readyz", ready)
	for _, target := range []string{
		"/", "/login", "/connections/", "/assets/app.js", "/assets/missing.js",
		"/api", "/api?x=1", "/api/", "/api/missing", "/api/readyz",
		"/api/readyz/", "/api/items/abc", "/api/items/abc/", "/api/items/a%2Fb",
		"/api/items/%61bc", "/api/items/%252F", "/api/items/%2e%2e",
		"/api//readyz?x=1", "/api/../login", "//login", "/connections/../login",
	} {
		for _, method := range []string{"GET", "HEAD", "POST", "DELETE", "OPTIONS"} {
			t.Run(method+" "+target, func(t *testing.T) {
				oldResponse, newResponse := httptest.NewRecorder(), httptest.NewRecorder()
				oldRoot.ServeHTTP(oldResponse, httptest.NewRequest(method, target, nil))
				newRoot.ServeHTTP(newResponse, httptest.NewRequest(method, target, nil))
				if newResponse.Code != oldResponse.Code || newResponse.Body.String() != oldResponse.Body.String() {
					t.Fatalf("got %d %q; previous router returned %d %q", newResponse.Code, newResponse.Body, oldResponse.Code, oldResponse.Body)
				}
				for _, header := range []string{"Allow", "Location", "X-Request-Method", "Content-Type", "Cache-Control"} {
					if newResponse.Header().Get(header) != oldResponse.Header().Get(header) {
						t.Errorf("%s: got %q; previous router returned %q", header, newResponse.Header().Get(header), oldResponse.Header().Get(header))
					}
				}
			})
		}
	}
}
