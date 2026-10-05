package routing

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/auth"
)

func (f routingFixture) request(method, path string, body io.Reader, cookie *http.Cookie) *http.Request {
	r := httptest.NewRequest(method, path, body)
	r.Header.Set("Origin", browserOrigin)
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	return r
}

func (f routingFixture) serve(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func TestSharedRoutesRejectExpiredAndDeletedAccounts(t *testing.T) {
	for _, kind := range []string{"expired", "deleted"} {
		t.Run(kind, func(t *testing.T) {
			f := newRoutingFixture(t)
			if kind == "expired" {
				ctx, err := f.sessions.Load(t.Context(), f.cookie.Value)
				if err != nil {
					t.Fatal(err)
				}
				f.sessions.SetDeadline(ctx, time.Now().Add(-time.Minute))
				if _, _, err := f.sessions.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := f.db.Exec("DELETE FROM accounts WHERE id = ?", f.owner); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{"/api/auth/me", "/api/keys", "/api/connections"} {
				w := f.serve(f.request("GET", path, nil, f.cookie))
				if w.Code != 401 {
					t.Fatalf("%s: %d %s", path, w.Code, w.Body)
				}
			}
			body := &unreadBody{}
			r := f.request("POST", "/api/keys", nil, f.cookie)
			r.Body = body
			if w := f.serve(r); w.Code != 401 || body.reads != 0 {
				t.Fatalf("upload: %d, reads %d", w.Code, body.reads)
			}
			w := f.serve(f.request("POST", "/api/auth/logout", strings.NewReader(`{}`), f.cookie))
			if w.Code != 204 {
				t.Fatalf("logout: %d %s", w.Code, w.Body)
			}
			cookies := w.Result().Cookies()
			if len(cookies) != 1 || cookies[0].MaxAge != -1 {
				t.Fatal("logout did not expire browser cookie")
			}
		})
	}
}

func TestSharedLogoutWaitsForInvalidation(t *testing.T) {
	f := newRoutingFixture(t)
	entered := make(chan auth.LoginSession, 1)
	release := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 1)
	invalidate := f.access.OnLogout
	f.access.OnLogout = func(session auth.LoginSession) {
		invalidate(session)
		entered <- session
		<-release
	}
	defer close(release)
	go func() { done <- f.serve(f.request("POST", "/api/auth/logout", strings.NewReader(`{}`), f.cookie)) }()
	select {
	case session := <-entered:
		digest := sha256.Sum256([]byte(f.cookie.Value))
		if session.ID != hex.EncodeToString(digest[:]) || session.ExpiresAt.IsZero() {
			t.Fatal("invalid login ownership passed to invalidation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("logout never invalidated the login")
	}
	// Session deletion must precede the callback, and HTTP success must follow it.
	if w := f.serve(f.request("GET", "/api/auth/me", nil, f.cookie)); w.Code != 401 {
		t.Fatalf("session survived logout: %d", w.Code)
	}
	select {
	case <-done:
		t.Fatal("logout returned before invalidation finished")
	case <-time.After(30 * time.Millisecond):
	}
	release <- struct{}{}
	select {
	case w := <-done:
		if w.Code != 204 {
			t.Fatalf("logout: %d %s", w.Code, w.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("logout did not return after invalidation")
	}
}

func TestSharedRoutesRejectOriginsBeforeReadingBodies(t *testing.T) {
	f := newRoutingFixture(t)
	for _, endpoint := range []struct{ method, path string }{
		{"POST", "/api/auth/register/begin"}, {"POST", "/api/auth/register/finish"},
		{"POST", "/api/auth/login/begin"}, {"POST", "/api/auth/login/finish"}, {"POST", "/api/auth/logout"},
		{"POST", "/api/keys"}, {"DELETE", "/api/keys/missing"},
		{"POST", "/api/connections"}, {"PUT", "/api/connections/missing"}, {"DELETE", "/api/connections/missing"},
		{"POST", "/api/connections/import/preview"}, {"POST", "/api/connections/import/confirm"},
		{"POST", "/api/connections/missing/host-key"}, {"POST", "/api/connections/missing/host-trust"}, {"POST", "/api/connections/missing/host-trust/reset"},
	} {
		for _, origin := range []string{"", "http://localhost:5174", browserOrigin + "/"} {
			t.Run(endpoint.method+endpoint.path+"/"+origin, func(t *testing.T) {
				body := &unreadBody{}
				r := f.request(endpoint.method, endpoint.path, nil, f.cookie)
				r.Body = body
				r.Header.Set("Origin", origin)
				w := f.serve(r)
				if w.Code != 403 || body.reads != 0 {
					t.Fatalf("got %d, reads %d: %s", w.Code, body.reads, w.Body)
				}
			})
		}
	}
}

func TestSharedPasskeyFinishConsumesChallengeBeforeParsing(t *testing.T) {
	for _, ceremony := range []string{"register", "login"} {
		for _, malformed := range []string{`{"secret":"do-not-echo"`, `null`, strings.Repeat(" ", 64*1024) + `{}`} {
			t.Run(ceremony+"/"+strconv.Itoa(len(malformed)), func(t *testing.T) {
				f := newRoutingFixture(t)
				body := `{}`
				if ceremony == "register" {
					body = `{"display_name":"New user"}`
				}
				prefix := "/api/auth/" + ceremony
				begin := f.serve(f.request("POST", prefix+"/begin", strings.NewReader(body), nil))
				if begin.Code != 200 {
					t.Fatalf("begin: %d %s", begin.Code, begin.Body)
				}
				cookies := begin.Result().Cookies()
				if len(cookies) != 1 {
					t.Fatal("missing ceremony cookie")
				}
				bad := f.serve(f.request("POST", prefix+"/finish", strings.NewReader(malformed), cookies[0]))
				if bad.Code != 400 || strings.Contains(bad.Body.String(), "do-not-echo") {
					t.Fatalf("unsafe parsing error: %d %s", bad.Code, bad.Body)
				}
				unread := &unreadBody{}
				replay := f.request("POST", prefix+"/finish", nil, cookies[0])
				replay.Body = unread
				w := f.serve(replay)
				if w.Code != 400 || !strings.Contains(w.Body.String(), "missing or expired") || unread.reads != 0 {
					t.Fatalf("challenge replay: %d, reads %d: %s", w.Code, unread.reads, w.Body)
				}
			})
		}
	}
}

func TestSharedJSONGuardsRunAfterAuthenticationBeforeParsing(t *testing.T) {
	f := newRoutingFixture(t)
	for _, endpoint := range []struct{ method, path string }{
		{"POST", "/api/connections"}, {"PUT", "/api/connections/missing"},
		{"POST", "/api/connections/import/preview"}, {"POST", "/api/connections/import/confirm"},
		{"POST", "/api/connections/missing/host-trust"}, {"POST", "/api/connections/missing/host-trust/reset"},
	} {
		t.Run(endpoint.method+endpoint.path, func(t *testing.T) {
			for _, cookie := range []*http.Cookie{nil, f.cookie} {
				body := &unreadBody{}
				r := f.request(endpoint.method, endpoint.path, nil, cookie)
				r.Header.Set("Content-Type", "text/plain")
				r.Body = body
				w := f.serve(r)
				want := 401
				if cookie != nil {
					want = 415
				}
				if w.Code != want || body.reads != 0 {
					t.Fatalf("got %d, want %d; reads %d: %s", w.Code, want, body.reads, w.Body)
				}
			}
		})
	}
}
