package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2/memstore"
	"github.com/carlyleec/go-ssh-term/internal/auth/sqlitestore"
	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestAuthConfiguration(t *testing.T) {
	for _, origin := range []string{"http://localhost:8080", "http://localhost:5173", "https://localhost:8443"} {
		t.Run(origin, func(t *testing.T) {
			t.Setenv("DATABASE_PATH", "/data/gateway.db")
			t.Setenv("ENCRYPTION_KEY_PATH", "/key-material/application.key")
			t.Setenv("HTTP_ADDR", ":8080")
			t.Setenv("SHUTDOWN_TIMEOUT", "5s")
			t.Setenv("BROWSER_ORIGIN", origin)
			t.Setenv("SESSION_LIFETIME", "12h")
			t.Setenv("CHALLENGE_LIFETIME", "5m")
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			wa, err := NewWebAuthn(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(wa.Config.RPOrigins) != 1 || wa.Config.RPOrigins[0] != origin || wa.Config.RPAllowCrossOrigin {
				t.Fatal("expected only the configured browser origin")
			}
			before := time.Now()
			registration, pendingRegistration, err := wa.BeginRegistration(testUser{})
			if err != nil {
				t.Fatal(err)
			}
			selection := registration.Response.AuthenticatorSelection
			if registration.Response.RelyingParty.ID != "localhost" || selection.ResidentKey != protocol.ResidentKeyRequirementRequired || selection.UserVerification != protocol.VerificationRequired {
				t.Fatal("registration must require a discoverable, user-verified credential for localhost")
			}
			login, pendingLogin, err := wa.BeginDiscoverableLogin()
			if err != nil {
				t.Fatal(err)
			}
			if login.Response.RelyingPartyID != "localhost" || len(login.Response.AllowedCredentials) != 0 || login.Response.UserVerification != protocol.VerificationRequired {
				t.Fatal("login must allow passkey account selection and require user verification")
			}
			for _, pending := range []*webauthn.SessionData{pendingRegistration, pendingLogin} {
				if pending.Challenge == "" || pending.Expires.Before(before.Add(5*time.Minute)) || pending.Expires.After(time.Now().Add(5*time.Minute)) {
					t.Fatal("ceremony must have a challenge and server-side five-minute deadline")
				}
			}

			// Exercise cookies without requiring the schema that will back the store.
			sessions, stop := NewSessions(cfg, nil)
			defer stop()
			if _, ok := sessions.Store.(*sqlitestore.Store); !ok {
				t.Fatal("expected SQLite store")
			}
			memory := memstore.NewWithCleanupInterval(0)
			sessions.Store = memory
			response := httptest.NewRecorder()
			sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sessions.Put(r.Context(), "account_id", "test-account")
			})).ServeHTTP(response, httptest.NewRequest("GET", origin, nil))
			cookies := response.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("expected one cookie, got %d", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != "ssh_term_session" || cookie.Value == "" || !cookie.HttpOnly || cookie.Secure != cfg.CookieSecure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
				t.Fatalf("unexpected session cookie: %s", cookie)
			}
			if cookie.MaxAge < 43199 || cookie.MaxAge > 43200 || sessions.IdleTimeout != 0 {
				t.Fatal("expected a twelve-hour session without an idle timeout")
			}
		})
	}
}

type testUser struct{}

func (testUser) WebAuthnID() []byte                         { return []byte("test-account") }
func (testUser) WebAuthnName() string                       { return "Test" }
func (testUser) WebAuthnDisplayName() string                { return "Test" }
func (testUser) WebAuthnCredentials() []webauthn.Credential { return nil }
