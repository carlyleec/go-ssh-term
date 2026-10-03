package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// LoginSession identifies a login without exposing its bearer cookie. Terminal
// owners must enforce ExpiresAt independently of HTTP activity or store cleanup.
type LoginSession struct {
	ID        string
	ExpiresAt time.Time
}

type loginSessionContextKey struct{}

func SessionFromContext(ctx context.Context) (LoginSession, bool) {
	session, ok := ctx.Value(loginSessionContextKey{}).(LoginSession)
	return session, ok
}

func (a *Access) loginSession(ctx context.Context) LoginSession {
	token := a.sessions.Token(ctx)
	if token == "" {
		return LoginSession{}
	}
	sum := sha256.Sum256([]byte(token))
	return LoginSession{ID: hex.EncodeToString(sum[:]), ExpiresAt: a.sessions.Deadline(ctx)}
}

// Logout deliberately does not require an existing account: expired, anonymous,
// and deleted-account sessions must still be able to clear their browser cookie.
func (a *Access) Logout() http.Handler {
	return authRequest(a.origin, loadSession(a.sessions, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := a.loginSession(r.Context())
		if err := a.sessions.Destroy(r.Context()); err != nil {
			authError(w, http.StatusServiceUnavailable, "could not sign out; try again")
			return
		}
		// The future terminal registry must synchronously revoke this owner before
		// success, coordinating invalidation with in-flight connection publication.
		if session.ID != "" && a.OnLogout != nil {
			a.OnLogout(session)
		}
		a.sessions.WriteSessionCookie(r.Context(), w, "", time.Time{})
		w.WriteHeader(http.StatusNoContent)
	})))
}
