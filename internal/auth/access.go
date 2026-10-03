package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/google/uuid"
)

type Account struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type accountContextKey struct{}

// AccountFromContext returns the identity verified by Require. Resource handlers
// must use its ID for ownership checks rather than a client-supplied account ID.
func AccountFromContext(ctx context.Context) (Account, bool) {
	account, ok := ctx.Value(accountContextKey{}).(Account)
	return account, ok
}

type Access struct {
	// OnLogout is configured at startup and called after session deletion, before
	// success. It must be concurrency-safe and idempotent for repeated requests.
	OnLogout func(LoginSession)
	sessions *scs.SessionManager
	origin   string
	lookup   func(context.Context, uuid.UUID) (Account, error)
}

func NewAccess(sessions *scs.SessionManager, pool *sql.DB, rpID, origin string) *Access {
	return &Access{sessions: sessions, origin: origin, lookup: func(ctx context.Context, id uuid.UUID) (Account, error) {
		ctx, cancel := sqlite.WorkContext(ctx)
		defer cancel()
		row, err := queries.New(pool).GetSessionAccount(ctx, queries.GetSessionAccountParams{ID: id.String(), RpID: rpID})
		if err != nil {
			return Account{}, err
		}
		return Account{ID: row.ID, DisplayName: row.DisplayName}, nil
	}}
}

// Require checks the session and account on every request. Loading a session does
// not extend its absolute deadline or create a cookie for anonymous requests.
func (a *Access) Require(next http.Handler) http.Handler {
	authenticated := loadSession(a.sessions, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(a.sessions.GetString(r.Context(), accountIDKey))
		if err != nil {
			authError(w, http.StatusUnauthorized, "sign in to continue")
			return
		}
		account, err := a.lookup(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			authError(w, http.StatusUnauthorized, "sign in to continue")
			return
		}
		if err != nil {
			authError(w, http.StatusServiceUnavailable, "could not load account; try again")
			return
		}
		ctx := context.WithValue(r.Context(), accountContextKey{}, account)
		ctx = context.WithValue(ctx, loginSessionContextKey{}, a.loginSession(r.Context()))
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
	guarded := RequireOrigin(a.origin, authenticated)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// WebSocket handshakes are GET requests but still require an exact origin.
		if (r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions) || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			guarded.ServeHTTP(w, r)
			return
		}
		authenticated.ServeHTTP(w, r)
	})
}

func (a *Access) CurrentUser() http.Handler {
	return a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, _ := AccountFromContext(r.Context())
		writeAuthJSON(w, map[string]Account{"account": account})
	}))
}
