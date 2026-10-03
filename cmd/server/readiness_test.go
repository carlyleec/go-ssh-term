package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestReadiness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		files  fstest.MapFS
		status int
	}{
		{"ready", nil, fstest.MapFS{"index.html": {Data: []byte("app")}}, http.StatusNoContent},
		{"database unavailable", errors.New("private database details"), fstest.MapFS{"index.html": {}}, http.StatusServiceUnavailable},
		{"frontend missing", nil, fstest.MapFS{}, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ping := func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 2*time.Second {
					t.Error("database probe needs a bounded deadline")
				}
				return tc.err
			}
			response := httptest.NewRecorder()
			readiness(ping, tc.files).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/readyz", nil))
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if strings.Contains(response.Body.String(), "private") {
				t.Fatal("probe exposed database details")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("readiness must not be cached")
			}
		})
	}
}

func TestReadinessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ping := func(ctx context.Context) error {
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Error("probe did not inherit request cancellation")
		}
		return ctx.Err()
	}
	response := httptest.NewRecorder()
	readiness(ping, fstest.MapFS{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/readyz", nil).WithContext(ctx))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}
