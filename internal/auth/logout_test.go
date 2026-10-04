package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/google/uuid"
)

func logoutRequest(handler http.Handler, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/auth/logout", strings.NewReader(`{}`))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestLogoutInvalidatesOnlyCurrentSession(t *testing.T) {
	h, _ := registrationFixture(t)
	id := uuid.NewString()
	first := accessCookie(t, h.sessions, id, false)
	other := accessCookie(t, h.sessions, id, false)
	a := &Access{sessions: h.sessions, origin: testOrigin}
	ctx, _ := h.sessions.Load(context.Background(), first.Value)
	owner := a.loginSession(ctx)
	var invalidated LoginSession
	a.OnLogout = func(session LoginSession) {
		invalidated = session
		if sessionAccount(t, h, first) != "" {
			t.Error("callback ran before deletion")
		}
	}
	w := logoutRequest(logoutHandler(a), first, testOrigin)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("logout: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "" || cookies[0].MaxAge != -1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" {
		t.Fatalf("cookie not cleared correctly: %v", cookies)
	}
	if invalidated != owner || owner.ID == "" || owner.ID == first.Value {
		t.Fatal("invalid session cleanup identity")
	}
	if sessionAccount(t, h, first) != "" || sessionAccount(t, h, other) != id {
		t.Fatal("logout did not isolate login sessions")
	}
	if again := logoutRequest(logoutHandler(a), first, testOrigin); again.Code != 204 {
		t.Fatalf("repeat logout: %d", again.Code)
	}
}

func TestLogoutFailuresAndAnonymousSessions(t *testing.T) {
	for _, kind := range []string{"missing", "expired", "anonymous", "origin", "load", "delete"} {
		t.Run(kind, func(t *testing.T) {
			h, _ := registrationFixture(t)
			a := &Access{sessions: h.sessions, origin: testOrigin}
			id := uuid.NewString()
			if kind == "anonymous" {
				id = ""
			}
			cookie := accessCookie(t, h.sessions, id, kind == "expired")
			if kind == "missing" {
				cookie = nil
			}
			store := h.sessions.Store
			h.sessions.Store = &failingSessionStore{Store: store, failFind: kind == "load", failDelete: kind == "delete"}
			called := false
			a.OnLogout = func(LoginSession) { called = true }
			origin := testOrigin
			if kind == "origin" {
				origin = "https://evil.example"
			}
			w := logoutRequest(logoutHandler(a), cookie, origin)
			want := 204
			if kind == "origin" {
				want = 403
			}
			if kind == "load" || kind == "delete" {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if want != 204 {
				var body map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				message, ok := body["error"].(string)
				if len(body) != 1 || !ok || message == "" || w.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("unexpected error response: %v %s", w.Header(), w.Body.String())
				}
				if len(w.Result().Cookies()) != 0 || called {
					t.Fatal("failure claimed invalidation")
				}
				h.sessions.Store = store
				if sessionAccount(t, h, cookie) != id {
					t.Fatal("failure removed session")
				}
			}
		})
	}
}

func TestLogoutConcurrent(t *testing.T) {
	h, _ := registrationFixture(t)
	cookie := accessCookie(t, h.sessions, uuid.NewString(), false)
	a := &Access{sessions: h.sessions, origin: testOrigin}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if w := logoutRequest(logoutHandler(a), cookie, testOrigin); w.Code != 204 {
				t.Errorf("concurrent logout: %d", w.Code)
			}
		})
	}
	wg.Wait()
	if sessionAccount(t, h, cookie) != "" {
		t.Fatal("session survived logout")
	}
}

func TestSessionBoundaryAndAbsoluteExpiry(t *testing.T) {
	h, _ := registrationFixture(t)
	id := uuid.New()
	cookie := accessCookie(t, h.sessions, id.String(), false)
	ctx, err := h.sessions.Load(context.Background(), cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	deadline := h.sessions.Deadline(ctx)
	a := &Access{sessions: h.sessions, origin: testOrigin, lookup: func(context.Context, uuid.UUID) (Account, error) { return Account{ID: id.String()}, nil }}
	var owner LoginSession
	handler := a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		owner, ok = SessionFromContext(r.Context())
		if !ok || owner.ID == cookie.Value || owner.ID == "" || !owner.ExpiresAt.Equal(deadline) {
			t.Error("incorrect session ownership boundary")
		}
		w.WriteHeader(204)
	}))
	r := httptest.NewRequest("GET", "/protected", nil)
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("active session: %d", w.Code)
	}
	after, _ := h.sessions.Load(context.Background(), cookie.Value)
	if !h.sessions.Deadline(after).Equal(deadline) {
		t.Fatal("read extended deadline")
	}
	h.sessions.SetDeadline(after, time.Now().Add(-time.Second))
	if _, _, err := h.sessions.Commit(after); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("expired session: %d", w.Code)
	}
	if _, ok := SessionFromContext(context.Background()); ok {
		t.Fatal("session outside protected context")
	}
}

func TestLogoutPersistedSession(t *testing.T) {
	pool := registrationDatabase(t)
	h, handler := persistedRegistrationFixture(t, pool)
	options, cookie := beginTest(t, handler, nil)
	finish := registrationRequestTest(handler, "finish", credentialResponse(t, options, testOrigin, "localhost", 0x45), testOrigin, cookie)
	if finish.Code != 201 {
		t.Fatalf("registration: %d", finish.Code)
	}
	cookie = finish.Result().Cookies()[0]
	a := NewAccess(h.sessions, pool, "localhost", testOrigin)
	if w := logoutRequest(logoutHandler(a), cookie, testOrigin); w.Code != 204 {
		t.Fatalf("logout: %d", w.Code)
	}
	r := httptest.NewRequest("GET", "/api/auth/me", nil)
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	currentUserHandler(a).ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("old cookie: %d", w.Code)
	}
}

func logoutHandler(access *Access) http.Handler {
	mux := http.NewServeMux()
	access.RegisterLogout(api.New(mux))
	return mux
}

func TestLogoutRequestContract(t *testing.T) {
	for _, test := range []struct {
		method, contentType, body string
		status                    int
	}{
		{"GET", "application/json", "{}", 405},
		{"POST", "", "{}", 415},
		{"POST", "text/plain", "{}", 415},
		{"POST", "application/json", "", 204},
		{"POST", "application/json; charset=utf-8", "ignored body", 204},
	} {
		t.Run(test.method+test.contentType+test.body, func(t *testing.T) {
			h, _ := registrationFixture(t)
			id := uuid.NewString()
			cookie := accessCookie(t, h.sessions, id, false)
			a := &Access{sessions: h.sessions, origin: testOrigin}
			r := httptest.NewRequest(test.method, "/api/auth/logout", strings.NewReader(test.body))
			r.Header.Set("Origin", testOrigin)
			r.Header.Set("Content-Type", test.contentType)
			r.AddCookie(cookie)
			w := httptest.NewRecorder()
			logoutHandler(a).ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatalf("status = %d: %s", w.Code, w.Body.String())
			}
			if test.status != 204 {
				if len(w.Result().Cookies()) != 0 || sessionAccount(t, h, cookie) != id {
					t.Fatal("rejected request invalidated session")
				}
				return
			}
			if w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" {
				t.Fatal("204 response has content")
			}
			if !reflect.DeepEqual(w.Header().Values("Cache-Control"), []string{"no-store", `no-cache="Set-Cookie"`}) {
				t.Fatalf("cache policy = %v", w.Header().Values("Cache-Control"))
			}
		})
	}
}

func TestLogoutCookieMatchesSessionConfiguration(t *testing.T) {
	h, _ := registrationFixture(t)
	h.sessions.Cookie.Name = "custom_session"
	h.sessions.Cookie.Domain = "example.test"
	h.sessions.Cookie.Path = "/gateway"
	h.sessions.Cookie.Secure = true
	h.sessions.Cookie.Partitioned = true
	a := &Access{sessions: h.sessions, origin: testOrigin}
	want := httptest.NewRecorder()
	h.sessions.WriteSessionCookie(context.Background(), want, "", time.Time{})
	got := logoutRequest(logoutHandler(a), nil, testOrigin)
	if !reflect.DeepEqual(got.Header().Values("Set-Cookie"), want.Header().Values("Set-Cookie")) {
		t.Fatalf("cookie = %v, want %v", got.Header().Values("Set-Cookie"), want.Header().Values("Set-Cookie"))
	}
}

func TestLogoutContract(t *testing.T) {
	contract := api.New(http.NewServeMux())
	var access Access
	access.RegisterLogout(contract)
	operation := contract.OpenAPI().Paths["/api/auth/logout"].Post
	if operation.OperationID != "logout" || len(operation.Security) != 0 {
		t.Fatal("logout requires no verified account")
	}
	success := operation.Responses["204"]
	if success == nil || len(success.Content) != 0 || success.Headers["Set-Cookie"] == nil {
		t.Fatal("missing empty success or cookie contract")
	}
	for _, status := range []string{"403", "415", "503"} {
		response := operation.Responses[status]
		if response == nil || response.Content["application/json"].Schema.Ref != "#/components/schemas/AuthErrorBody" {
			t.Fatalf("missing error contract for %s", status)
		}
	}
}
