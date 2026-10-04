package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alexedwards/scs/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	driver "modernc.org/sqlite"
	"modernc.org/sqlite/lib"
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
func NewRegistration(wa *webauthn.WebAuthn, sessions *scs.SessionManager, pool *sql.DB) *registration {
	h := &registration{webauthn: wa, sessions: sessions, pending: make(map[string]pendingRegistration)}
	h.save = func(ctx context.Context, user registrationUser, credential *webauthn.Credential) error {
		return saveRegistration(ctx, pool, wa.Config.RPID, user, credential)
	}
	return h
}

func (h *registration) begin(ctx context.Context, input *RegistrationBeginInput) (*RegistrationBeginOutput, error) {
	r, w := input.request, input.writer

	name := strings.TrimSpace(input.Body.DisplayName)
	if name == "" || utf8.RuneCountInString(name) > 64 || strings.ContainsFunc(name, unicode.IsControl) {
		return nil, &AuthErrorBody{Message: "display name must contain 1 to 64 characters without control characters", status: http.StatusBadRequest}
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, &AuthErrorBody{Message: "could not begin registration", status: http.StatusInternalServerError}
	}
	handle := make([]byte, 32)
	if _, err := rand.Read(handle); err != nil {
		return nil, &AuthErrorBody{Message: "could not begin registration", status: http.StatusInternalServerError}
	}
	user := registrationUser{ID: id, Handle: handle, DisplayName: name}
	options, session, err := h.webauthn.BeginRegistration(user)
	if err != nil {
		return nil, &AuthErrorBody{Message: "could not begin registration", status: http.StatusInternalServerError}
	}
	binding := h.sessions.GetString(r.Context(), registrationBinding)
	if binding == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			return nil, &AuthErrorBody{Message: "could not begin registration", status: http.StatusInternalServerError}
		}
		binding = base64.RawURLEncoding.EncodeToString(token)
	}
	if !h.put(binding, pendingRegistration{user: user, session: *session}) {
		return nil, &AuthErrorBody{Message: "registration is busy; try again shortly", status: http.StatusServiceUnavailable}
	}
	h.sessions.Put(r.Context(), registrationBinding, binding)
	if err := commitSession(w, r, h.sessions); err != nil {
		return nil, &AuthErrorBody{Message: "could not save browser session; begin again", status: http.StatusServiceUnavailable}
	}
	return &RegistrationBeginOutput{Body: *options}, nil
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

func (h *registration) finish(ctx context.Context, input *RegistrationFinishInput) (*AccountOutput, error) {
	r, w := input.request, input.writer
	pending := ctx.Value(registrationCeremonyKey{}).(pendingRegistration)
	parsed, err := input.Body.Parse()
	if err != nil {
		return nil, &AuthErrorBody{Message: "passkey verification failed; begin again", status: http.StatusBadRequest}
	}
	credential, err := h.webauthn.CreateCredential(pending.user, pending.session, parsed)
	if err != nil {
		return nil, &AuthErrorBody{Message: "passkey verification failed; begin again", status: http.StatusBadRequest}
	}

	if err := h.save(r.Context(), pending.user, credential); err != nil {
		var sqliteErr *driver.Error
		if errors.As(err, &sqliteErr) && (sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY) {
			return nil, &AuthErrorBody{Message: "account or passkey is already registered; sign in instead", status: http.StatusConflict}
		} else {
			return nil, &AuthErrorBody{Message: "could not save registration; try signing in or begin again", status: http.StatusServiceUnavailable}
		}
	}
	if err := establishSession(w, r, h.sessions, pending.user.ID.String()); err != nil {
		return nil, &AuthErrorBody{Message: "account saved but session could not be started; sign in with your passkey", status: http.StatusServiceUnavailable}
	}
	return &AccountOutput{Body: AccountBody{Account: Account{ID: pending.user.ID.String(), DisplayName: pending.user.DisplayName}}}, nil
}

type RegistrationBeginInput struct {
	SessionRequest
	Body struct {
		DisplayName string `json:"display_name"`
	}
}
type RegistrationBeginOutput struct{ Body protocol.CredentialCreation }
type RegistrationFinishInput struct {
	SessionRequest
	Body protocol.CredentialCreationResponse
}
type registrationCeremonyKey struct{}

func (h *registration) Register(api huma.API, origin string) {
	registerPasskeySchemas(api)
	huma.Register(api, huma.Operation{
		OperationID: "beginRegistration", Method: http.MethodPost, Path: "/api/auth/register/begin",
		MaxBodyBytes: 4096,
		Responses:    authResponses(api, 400, 403, 409, 415, 500, 503),
		Middlewares:  huma.Middlewares{h.guard(origin, false)},
		Metadata:     map[string]any{"authBodyError": "invalid registration request"},
	}, h.begin)
	huma.Register(api, huma.Operation{
		OperationID: "finishRegistration", Method: http.MethodPost, Path: "/api/auth/register/finish",
		DefaultStatus: http.StatusCreated, MaxBodyBytes: 64 * 1024,
		// The WebAuthn parser owns credential validation, including extension data.
		SkipValidateBody: true,
		Responses:        authResponses(api, 400, 403, 409, 415, 503),
		Middlewares:      huma.Middlewares{h.guard(origin, true)},
		Metadata:         map[string]any{"authBodyError": "passkey verification failed; begin again"},
	}, h.finish)
}

func (h *registration) guard(origin string, finish bool) func(huma.Context, func(huma.Context)) {
	return humaMiddleware(func(next http.Handler) http.Handler {
		return authRequest(origin, loadSession(h.sessions, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h.sessions.GetString(r.Context(), accountIDKey) != "" {
				authError(w, 409, "already signed in; log out before creating another account")
				return
			}
			if finish {
				// Consume before Huma parses the body, including malformed requests.
				pending, ok := h.take(h.sessions.GetString(r.Context(), registrationBinding))
				if !ok {
					authError(w, 400, "registration is missing or expired; begin again")
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), registrationCeremonyKey{}, pending))
			}
			next.ServeHTTP(w, r)
		})))
	})
}
