package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/database/queries"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const loginBinding = "login_binding"
const maxPendingLogins = 1024

var errInvalidLogin = errors.New("invalid passkey login")

type login struct {
	webauthn *webauthn.WebAuthn
	sessions *scs.SessionManager
	verify   func(context.Context, webauthn.SessionData, *protocol.ParsedCredentialAssertionData) (queries.Account, error)
	mu       sync.Mutex
	pending  map[string]webauthn.SessionData
}

func NewLogin(wa *webauthn.WebAuthn, sessions *scs.SessionManager, pool *pgxpool.Pool, origin string) http.Handler {
	h := &login{webauthn: wa, sessions: sessions, pending: make(map[string]webauthn.SessionData)}
	h.verify = func(ctx context.Context, session webauthn.SessionData, assertion *protocol.ParsedCredentialAssertionData) (queries.Account, error) {
		return verifyLogin(ctx, pool, wa, session, assertion)
	}
	return h.routes(origin)
}

func (h *login) routes(origin string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/auth/login/begin", authRequest(origin, loadSession(h.sessions, http.HandlerFunc(h.begin))))
	mux.Handle("POST /api/auth/login/finish", authRequest(origin, loadSession(h.sessions, http.HandlerFunc(h.finish))))
	return mux
}

func (h *login) begin(w http.ResponseWriter, r *http.Request) {
	if h.sessions.GetString(r.Context(), accountIDKey) != "" {
		authError(w, http.StatusConflict, "already signed in; log out before signing in again")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil || input == nil || len(input) != 0 {
		authError(w, http.StatusBadRequest, "login begin expects an empty JSON object")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		authError(w, http.StatusBadRequest, "request must contain one JSON object")
		return
	}
	options, session, err := h.webauthn.BeginDiscoverableLogin()
	if err != nil {
		authError(w, http.StatusInternalServerError, "could not begin login")
		return
	}
	binding := h.sessions.GetString(r.Context(), loginBinding)
	if binding == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			authError(w, http.StatusInternalServerError, "could not begin login")
			return
		}
		binding = base64.RawURLEncoding.EncodeToString(token)
	}
	if !h.put(binding, *session) {
		authError(w, http.StatusServiceUnavailable, "login is busy; try again shortly")
		return
	}
	h.sessions.Put(r.Context(), loginBinding, binding)
	if err := commitSession(w, r, h.sessions); err != nil {
		authError(w, http.StatusServiceUnavailable, "could not save browser session; begin again")
		return
	}
	writeAuthJSON(w, options)
}

func (h *login) put(binding string, session webauthn.SessionData) bool {
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
func (h *login) take(binding string) (webauthn.SessionData, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	session, ok := h.pending[binding]
	delete(h.pending, binding)
	return session, ok && time.Now().Before(session.Expires)
}

func (h *login) finish(w http.ResponseWriter, r *http.Request) {
	if h.sessions.GetString(r.Context(), accountIDKey) != "" {
		authError(w, http.StatusConflict, "already signed in; log out before signing in again")
		return
	}
	session, ok := h.take(h.sessions.GetString(r.Context(), loginBinding))
	if !ok {
		authError(w, http.StatusBadRequest, "login is missing or expired; begin again")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	assertion, err := protocol.ParseCredentialRequestResponse(r)
	if err != nil {
		authError(w, http.StatusBadRequest, "invalid passkey response; begin again")
		return
	}
	account, err := h.verify(r.Context(), session, assertion)
	if err != nil {
		if errors.Is(err, errInvalidLogin) {
			authError(w, http.StatusUnauthorized, "passkey verification failed; begin again")
		} else {
			authError(w, http.StatusServiceUnavailable, "could not complete login; begin again")
		}
		return
	}
	id := uuid.UUID(account.ID.Bytes).String()
	if err := establishSession(w, r, h.sessions, id); err != nil {
		authError(w, http.StatusServiceUnavailable, "could not start authenticated session; begin login again")
		return
	}
	writeAuthJSON(w, map[string]any{"account": map[string]string{"id": id, "display_name": account.DisplayName}})
}
