package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const loginBinding = "login_binding"
const maxPendingLogins = 1024

var errInvalidLogin = errors.New("invalid passkey login")

type Login struct {
	webauthn *webauthn.WebAuthn
	sessions *scs.SessionManager
	verify   func(context.Context, webauthn.SessionData, *protocol.ParsedCredentialAssertionData) (queries.Account, error)
	mu       sync.Mutex
	pending  map[string]webauthn.SessionData
}

func NewLogin(wa *webauthn.WebAuthn, sessions *scs.SessionManager, pool *sql.DB) *Login {
	h := &Login{webauthn: wa, sessions: sessions, pending: make(map[string]webauthn.SessionData)}
	h.verify = func(ctx context.Context, session webauthn.SessionData, assertion *protocol.ParsedCredentialAssertionData) (queries.Account, error) {
		return verifyLogin(ctx, pool, wa, session, assertion)
	}
	return h
}

func (h *Login) Begin(ctx context.Context, input *LoginBeginInput) (*LoginBeginOutput, error) {
	r, w := input.request, input.writer

	options, session, err := h.webauthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, &AuthErrorBody{Message: "could not begin login", status: http.StatusInternalServerError}
	}
	binding := h.sessions.GetString(r.Context(), loginBinding)
	if binding == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			return nil, &AuthErrorBody{Message: "could not begin login", status: http.StatusInternalServerError}
		}
		binding = base64.RawURLEncoding.EncodeToString(token)
	}
	if !h.put(binding, *session) {
		return nil, &AuthErrorBody{Message: "login is busy; try again shortly", status: http.StatusServiceUnavailable}
	}
	h.sessions.Put(r.Context(), loginBinding, binding)
	if err := commitSession(w, r, h.sessions); err != nil {
		return nil, &AuthErrorBody{Message: "could not save browser session; begin again", status: http.StatusServiceUnavailable}
	}
	return &LoginBeginOutput{Body: *options}, nil
}

func (h *Login) put(binding string, session webauthn.SessionData) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for key, value := range h.pending {
		if !now.Before(value.Expires) {
			delete(h.pending, key)
		}
	}
	if _, exists := h.pending[binding]; !exists && len(h.pending) >= maxPendingLogins {
		return false
	}
	h.pending[binding] = session
	return true
}
func (h *Login) take(binding string) (webauthn.SessionData, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	session, ok := h.pending[binding]
	delete(h.pending, binding)
	return session, ok && time.Now().Before(session.Expires)
}

func (h *Login) Finish(ctx context.Context, input *LoginFinishInput) (*AccountOutput, error) {
	r, w := input.request, input.writer
	session := ctx.Value(loginCeremonyKey{}).(webauthn.SessionData)
	assertion, err := input.Body.Parse()
	if err != nil {
		return nil, &AuthErrorBody{Message: "invalid passkey response; begin again", status: http.StatusBadRequest}
	}

	account, err := h.verify(r.Context(), session, assertion)
	if err != nil {
		if errors.Is(err, errInvalidLogin) {
			return nil, &AuthErrorBody{Message: "passkey verification failed; begin again", status: http.StatusUnauthorized}
		} else {
			return nil, &AuthErrorBody{Message: "could not complete login; begin again", status: http.StatusServiceUnavailable}
		}
	}
	id := account.ID
	if err := establishSession(w, r, h.sessions, id); err != nil {
		return nil, &AuthErrorBody{Message: "could not start authenticated session; begin login again", status: http.StatusServiceUnavailable}
	}
	return &AccountOutput{Body: AccountBody{Account: Account{ID: id, DisplayName: account.DisplayName}}}, nil
}

type LoginBeginInput struct {
	SessionRequest
	Body struct{}
}
type LoginBeginOutput struct{ Body protocol.CredentialAssertion }
type LoginFinishInput struct {
	SessionRequest
	Body protocol.CredentialAssertionResponse
}
type loginCeremonyKey struct{}

func (h *Login) guard(origin string, finish bool) func(huma.Context, func(huma.Context)) {
	return humaMiddleware(func(next http.Handler) http.Handler {
		return authRequest(origin, loadSession(h.sessions, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h.sessions.GetString(r.Context(), accountIDKey) != "" {
				authError(w, 409, "already signed in; log out before signing in again")
				return
			}
			if finish {
				session, ok := h.take(h.sessions.GetString(r.Context(), loginBinding))
				if !ok {
					authError(w, 400, "login is missing or expired; begin again")
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), loginCeremonyKey{}, session))
			}
			next.ServeHTTP(w, r)
		})))
	})
}

// BeginMiddleware validates the browser session before starting a ceremony.
func (h *Login) BeginMiddleware(origin string) func(huma.Context, func(huma.Context)) {
	return h.guard(origin, false)
}

// FinishMiddleware consumes the pending challenge before Huma parses the body.
func (h *Login) FinishMiddleware(origin string) func(huma.Context, func(huma.Context)) {
	return h.guard(origin, true)
}
