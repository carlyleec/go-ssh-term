package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const registrationBinding = "registration_binding"
const maxPendingRegistrations = 1024

type registrationUser struct {
	ID          uuid.UUID
	Handle      []byte
	DisplayName string
}

func (u registrationUser) WebAuthnID() []byte                         { return u.Handle }
func (u registrationUser) WebAuthnName() string                       { return u.DisplayName }
func (u registrationUser) WebAuthnDisplayName() string                { return u.DisplayName }
func (u registrationUser) WebAuthnCredentials() []webauthn.Credential { return nil }

type pendingRegistration struct {
	user    registrationUser
	session webauthn.SessionData
}

type registration struct {
	webauthn *webauthn.WebAuthn
	sessions *scs.SessionManager
	save     func(context.Context, registrationUser, *webauthn.Credential) error
	mu       sync.Mutex
	pending  map[string]pendingRegistration
}

// NewRegistration keeps one pending ceremony per browser session. Restarting
// the process invalidates pending ceremonies, while login sessions remain stored.
func NewRegistration(wa *webauthn.WebAuthn, sessions *scs.SessionManager, pool *pgxpool.Pool, origin string) http.Handler {
	h := &registration{webauthn: wa, sessions: sessions, pending: make(map[string]pendingRegistration)}
	h.save = func(ctx context.Context, user registrationUser, credential *webauthn.Credential) error {
		return saveRegistration(ctx, pool, wa.Config.RPID, user, credential)
	}
	return h.routes(origin)
}

func (h *registration) routes(origin string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/auth/register/begin", authRequest(origin, h.sessions.LoadAndSave(http.HandlerFunc(h.begin))))
	mux.Handle("POST /api/auth/register/finish", authRequest(origin, loadSession(h.sessions, http.HandlerFunc(h.finish))))
	return mux
}

func (h *registration) begin(w http.ResponseWriter, r *http.Request) {
	if h.sessions.GetString(r.Context(), accountIDKey) != "" {
		authError(w, http.StatusConflict, "already signed in; log out before creating another account")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		DisplayName string `json:"display_name"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		authError(w, http.StatusBadRequest, "invalid registration request")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		authError(w, http.StatusBadRequest, "request must contain one JSON object")
		return
	}
	name := strings.TrimSpace(input.DisplayName)
	if name == "" || utf8.RuneCountInString(name) > 64 || strings.ContainsFunc(name, unicode.IsControl) {
		authError(w, http.StatusBadRequest, "display name must contain 1 to 64 characters without control characters")
		return
	}
	id, err := uuid.NewRandom()
	if err != nil {
		authError(w, http.StatusInternalServerError, "could not begin registration")
		return
	}
	handle := make([]byte, 32)
	if _, err := rand.Read(handle); err != nil {
		authError(w, http.StatusInternalServerError, "could not begin registration")
		return
	}
	user := registrationUser{ID: id, Handle: handle, DisplayName: name}
	options, session, err := h.webauthn.BeginRegistration(user)
	if err != nil {
		authError(w, http.StatusInternalServerError, "could not begin registration")
		return
	}
	binding := h.sessions.GetString(r.Context(), registrationBinding)
	if binding == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			authError(w, http.StatusInternalServerError, "could not begin registration")
			return
		}
		binding = base64.RawURLEncoding.EncodeToString(token)
	}
	if !h.put(binding, pendingRegistration{user: user, session: *session}) {
		authError(w, http.StatusServiceUnavailable, "registration is busy; try again shortly")
		return
	}
	h.sessions.Put(r.Context(), registrationBinding, binding)
	writeAuthJSON(w, options)
}

func (h *registration) put(binding string, pending pendingRegistration) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for key, value := range h.pending {
		if !now.Before(value.session.Expires) {
			delete(h.pending, key)
		}
	}
	if _, exists := h.pending[binding]; !exists && len(h.pending) >= maxPendingRegistrations {
		return false
	}
	h.pending[binding] = pending
	return true
}

func (h *registration) take(binding string) (pendingRegistration, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	pending, ok := h.pending[binding]
	// Consume before verification so parallel finish requests cannot both succeed.
	delete(h.pending, binding)
	return pending, ok && time.Now().Before(pending.session.Expires)
}

func (h *registration) finish(w http.ResponseWriter, r *http.Request) {
	if h.sessions.GetString(r.Context(), accountIDKey) != "" {
		authError(w, http.StatusConflict, "already signed in; log out before creating another account")
		return
	}

	binding := h.sessions.GetString(r.Context(), registrationBinding)
	pending, ok := h.take(binding)
	if !ok {
		authError(w, http.StatusBadRequest, "registration is missing or expired; begin again")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	credential, err := h.webauthn.FinishRegistration(pending.user, pending.session, r)
	if err != nil {
		authError(w, http.StatusBadRequest, "passkey verification failed; begin again")
		return
	}
	if err := h.save(r.Context(), pending.user, credential); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			authError(w, http.StatusConflict, "account or passkey is already registered; sign in instead")
		} else {
			authError(w, http.StatusServiceUnavailable, "could not save registration; try signing in or begin again")
		}
		return
	}
	if err := establishSession(w, r, h.sessions, pending.user.ID.String()); err != nil {
		authError(w, http.StatusServiceUnavailable, "account saved but session could not be started; sign in with your passkey")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeAuthJSON(w, map[string]any{"account": map[string]string{
		"id": pending.user.ID.String(), "display_name": pending.user.DisplayName,
	}})
}
