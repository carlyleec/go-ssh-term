package routing

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/carlyleec/go-ssh-term/internal/connections"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"github.com/google/uuid"
)

const browserOrigin = "http://localhost:5173"

func routeFixture(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	db, _ := testdb.New(t)
	cfg := config.Config{RPID: "localhost", BrowserOrigin: browserOrigin, SessionLifetime: time.Hour, ChallengeLifetime: time.Minute}
	sessions, stop := auth.NewSessions(cfg, db)
	t.Cleanup(stop)
	wa, err := auth.NewWebAuthn(cfg)
	if err != nil {
		t.Fatal(err)
	}
	access := auth.NewAccess(sessions, db, cfg.RPID, browserOrigin)
	mux := api.NewRouter()
	Register(api.New(mux), Handlers{
		Access: access, Registration: auth.NewRegistration(wa, sessions, db), Login: auth.NewLogin(wa, sessions, db),
		Keys: sshkeys.NewHandler(db, nil), Connections: connections.NewHandler(db), Dialer: connections.NewDialer(db, nil),
	}, browserOrigin)
	id := uuid.NewString()
	if _, err := db.Exec("INSERT INTO accounts VALUES (?, 'Owner', 'localhost', X'01', 1)", id); err != nil {
		t.Fatal(err)
	}
	ctx, err := sessions.Load(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	sessions.Put(ctx, "account_id", id)
	token, _, err := sessions.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return mux, &http.Cookie{Name: sessions.Cookie.Name, Value: token}
}

type unreadBody struct{ reads int }

func (b *unreadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("body read before authentication")
}
func (*unreadBody) Close() error { return nil }

func TestSharedRoutesRequireAccountBeforeBodyReads(t *testing.T) {
	handler, _ := routeFixture(t)
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/auth/me"}, {"GET", "/api/keys"}, {"POST", "/api/keys"}, {"DELETE", "/api/keys/missing"},
		{"GET", "/api/connections"}, {"POST", "/api/connections"}, {"PUT", "/api/connections/missing"}, {"DELETE", "/api/connections/missing"},
		{"POST", "/api/connections/import/preview"}, {"POST", "/api/connections/import/confirm"},
		{"POST", "/api/connections/missing/host-key"}, {"POST", "/api/connections/missing/host-trust"}, {"POST", "/api/connections/missing/host-trust/reset"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			body := &unreadBody{}
			r := httptest.NewRequest(route.method, route.path, nil)
			r.Body = body
			r.Header.Set("Origin", browserOrigin)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 401 || body.reads != 0 {
				t.Fatalf("got %d, %d body reads: %s", w.Code, body.reads, w.Body)
			}
		})
	}
}

func TestSharedRouteDispatch(t *testing.T) {
	handler, cookie := routeFixture(t)
	for _, tc := range []struct {
		method, path, body string
		signedIn           bool
		status             int
	}{
		{"GET", "/api/auth/me", "", true, 200},
		{"GET", "/api/keys", "", true, 200},
		{"GET", "/api/connections", "", true, 200},
		{"POST", "/api/auth/register/begin", `{"display_name":"New user"}`, false, 200},
		{"POST", "/api/auth/login/begin", `{}`, false, 200},
		{"POST", "/api/auth/register/finish", `{}`, false, 400},
		{"POST", "/api/auth/login/finish", `{}`, false, 400},
		{"POST", "/api/auth/logout", `{}`, false, 204},
		{"POST", "/api/connections", `{}`, true, 400},
		{"DELETE", "/api/connections/missing", "", true, 404},
		{"DELETE", "/api/keys/missing", "", true, 404},
		{"POST", "/api/connections/missing/host-key", `{}`, true, 404},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Origin", browserOrigin)
			r.Header.Set("Content-Type", "application/json")
			if tc.signedIn {
				r.AddCookie(cookie)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.status, w.Body)
			}
		})
	}
}
