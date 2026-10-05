package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/google/uuid"
)

func accessCookie(t *testing.T, sessions *scs.SessionManager, id string, expired bool) *http.Cookie {
	t.Helper()
	ctx, err := sessions.Load(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		sessions.Put(ctx, accountIDKey, id)
	} else {
		sessions.Put(ctx, registrationBinding, "pending")
	}
	if expired {
		sessions.SetDeadline(ctx, time.Now().Add(-time.Minute))
	}
	token, _, err := sessions.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: sessions.Cookie.Name, Value: token}
}

func TestAccessSessionAuthority(t *testing.T) {
	for _, kind := range []string{"missing", "unknown", "anonymous", "malformed", "expired", "valid", "deleted-account", "database-failure", "store-failure"} {
		t.Run(kind, func(t *testing.T) {
			h, _ := registrationFixture(t)
			id := uuid.New()
			lookupCalls := 0
			access := &Access{sessions: h.sessions, origin: testOrigin, lookup: func(_ context.Context, got uuid.UUID) (Account, error) {
				lookupCalls++
				if got != id {
					t.Fatalf("lookup ID = %s", got)
				}
				if kind == "deleted-account" {
					return Account{}, sql.ErrNoRows
				}
				if kind == "database-failure" {
					return Account{}, errors.New("secret database failure")
				}
				return Account{ID: id.String(), DisplayName: "Alice"}, nil
			}}
			r := httptest.NewRequest("GET", "/api/auth/me", nil)
			switch kind {
			case "missing":
			case "unknown":
				r.AddCookie(&http.Cookie{Name: h.sessions.Cookie.Name, Value: "unknown"})
			case "anonymous":
				r.AddCookie(accessCookie(t, h.sessions, "", false))
			case "malformed":
				r.AddCookie(accessCookie(t, h.sessions, "not-a-uuid", false))
			default:
				r.AddCookie(accessCookie(t, h.sessions, id.String(), kind == "expired"))
			}
			if kind == "store-failure" {
				h.sessions.Store = &failingSessionStore{Store: h.sessions.Store, failFind: true}
			}
			w := httptest.NewRecorder()
			currentUserHandler(access).ServeHTTP(w, r)
			want := 401
			if kind == "valid" {
				want = 200
			}
			if kind == "database-failure" || kind == "store-failure" {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("status = %d, want %d: %s", w.Code, want, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Vary"), "Cookie") || len(w.Result().Cookies()) != 0 {
				t.Fatalf("unexpected headers: %v", w.Header())
			}
			if strings.Contains(w.Body.String(), "secret") {
				t.Fatal("private error leaked")
			}
			if want != http.StatusOK {
				var body map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				message, ok := body["error"].(string)
				if len(body) != 1 || !ok || message == "" {
					t.Fatalf("unexpected error envelope: %s", w.Body.String())
				}
			}
			if kind == "valid" && !strings.Contains(w.Body.String(), id.String()) {
				t.Fatal("missing account identity")
			}
			if want == 401 && kind != "deleted-account" && lookupCalls != 0 {
				t.Fatal("unauthenticated request reached account lookup")
			}
		})
	}
}

func TestAccessOriginAndContext(t *testing.T) {
	h, _ := registrationFixture(t)
	id := uuid.New()
	access := &Access{sessions: h.sessions, origin: testOrigin, lookup: func(context.Context, uuid.UUID) (Account, error) {
		return Account{ID: id.String(), DisplayName: "Alice"}, nil
	}}
	cookie := accessCookie(t, h.sessions, id.String(), false)
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "websocket"} {
		for _, origin := range []string{"", "https://evil.example", testOrigin} {
			t.Run(method+origin, func(t *testing.T) {
				requestMethod := method
				if method == "websocket" {
					requestMethod = "GET"
				}
				r := httptest.NewRequest(requestMethod, "/protected", nil)
				r.Header.Set("Origin", origin)
				if method == "websocket" {
					r.Header.Set("Upgrade", "websocket")
					r.Header.Set("Connection", "Upgrade")
				}
				r.AddCookie(cookie)
				w := httptest.NewRecorder()
				called := false
				access.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					account, ok := AccountFromContext(r.Context())
					if !ok || account.ID != id.String() {
						t.Fatal("missing verified identity")
					}
					w.WriteHeader(http.StatusNoContent)
				})).ServeHTTP(w, r)
				allowed := method == "GET" || origin == testOrigin
				if called != allowed {
					t.Fatalf("handler called = %v, allowed = %v", called, allowed)
				}
				if !allowed && w.Code != 403 {
					t.Fatalf("status = %d", w.Code)
				}
			})
		}
	}
	if _, ok := AccountFromContext(context.Background()); ok {
		t.Fatal("identity exists outside middleware")
	}
}

func TestCurrentUserPersistedAccount(t *testing.T) {
	pool := registrationDatabase(t)
	h, handler := persistedRegistrationFixture(t, pool)
	options, cookie := beginTest(t, handler, nil)
	finish := registrationRequestTest(handler, "finish", credentialResponse(t, options, testOrigin, "localhost", 0x45), testOrigin, cookie)
	if finish.Code != 201 {
		t.Fatalf("registration: %d %s", finish.Code, finish.Body.String())
	}
	cookie = finish.Result().Cookies()[0]
	for _, rp := range []string{"localhost", "other.example"} {
		r := httptest.NewRequest("GET", "/api/auth/me", nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		currentUserHandler(NewAccess(h.sessions, pool, rp, testOrigin)).ServeHTTP(w, r)
		want := 200
		if rp != "localhost" {
			want = 401
		}
		if w.Code != want {
			t.Fatalf("RP %s: %d %s", rp, w.Code, w.Body.String())
		}
		if want == 200 && !strings.Contains(w.Body.String(), "Alice") {
			t.Fatal("missing saved display name")
		}
	}
}

func currentUserHandler(access *Access) http.Handler {
	mux := api.NewRouter()
	access.RegisterCurrentUser(api.New(mux))
	return mux
}

func TestCurrentUserTypedResponseAndSessionDeadline(t *testing.T) {
	h, _ := registrationFixture(t)
	id := uuid.New()
	access := &Access{sessions: h.sessions, origin: testOrigin, lookup: func(context.Context, uuid.UUID) (Account, error) {
		return Account{ID: id.String(), DisplayName: "Alice"}, nil
	}}
	cookie := accessCookie(t, h.sessions, id.String(), false)
	before, err := h.sessions.Load(t.Context(), cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	deadline := h.sessions.Deadline(before)
	mux := currentUserHandler(access)
	r := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"account": map[string]any{"id": id.String(), "display_name": "Alice"}}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("response envelope changed: %s", w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Link") != "" {
		t.Fatalf("response headers changed: %v", w.Header())
	}
	after, err := h.sessions.Load(t.Context(), cookie.Value)
	if err != nil || !h.sessions.Deadline(after).Equal(deadline) {
		t.Fatalf("current-user changed session deadline: %v", err)
	}
	for _, method := range []string{http.MethodHead, http.MethodPost} {
		r := httptest.NewRequest(method, "/api/auth/me", nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		want := http.StatusOK
		if method == http.MethodPost {
			want = http.StatusMethodNotAllowed
		}
		if w.Code != want {
			t.Fatalf("%s: status %d, want %d", method, w.Code, want)
		}
	}
}

func TestCurrentUserContractMatchesRegistration(t *testing.T) {
	mux := api.NewRouter()
	contract := api.New(mux)
	var access Access
	access.RegisterCurrentUser(contract)
	op := contract.OpenAPI().Paths["/api/auth/me"].Get
	if op == nil || op.OperationID != "currentUser" {
		t.Fatal("missing current-user operation")
	}
	for _, status := range []string{"200", "401", "403", "503"} {
		response := op.Responses[status]
		if response == nil || response.Content["application/json"] == nil {
			t.Fatalf("missing JSON response for %s", status)
		}
	}
	if len(op.Security) != 1 || op.Security[0]["session"] == nil {
		t.Fatal("missing session-cookie security requirement")
	}
	h, _ := registrationFixture(t)
	if contract.OpenAPI().Components.SecuritySchemes["session"].Name != h.sessions.Cookie.Name {
		t.Fatal("documented session cookie differs from runtime")
	}
	for _, path := range []string{"/docs", "/schemas/Account.json", "/openapi.json"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("unexpected documentation route: %s", path)
		}
	}
}
