package auth

import (
	"encoding/json"
	"mime"
	"net/http"

	"github.com/alexedwards/scs/v2"
)

const accountIDKey = "account_id"

// Handlers using loadSession commit explicitly before sending success, so a
// session-store failure cannot be followed by a successful authentication body.
func loadSession(sessions *scs.SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")
		token := ""
		if cookie, err := r.Cookie(sessions.Cookie.Name); err == nil {
			token = cookie.Value
		}
		ctx, err := sessions.Load(r.Context(), token)
		if err != nil {
			authError(w, http.StatusServiceUnavailable, "could not load browser session; try again")
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authRequest(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Header.Get("Origin") != origin {
			authError(w, http.StatusForbidden, "request origin is not allowed")
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			authError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func writeAuthJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func commitSession(w http.ResponseWriter, r *http.Request, sessions *scs.SessionManager) error {
	token, expiry, err := sessions.Commit(r.Context())
	if err != nil {
		return err
	}
	sessions.WriteSessionCookie(r.Context(), w, token, expiry)
	return nil
}

func establishSession(w http.ResponseWriter, r *http.Request, sessions *scs.SessionManager, accountID string) error {
	// Destroy discards anonymous state and starts a full lifetime with a new token.
	if err := sessions.Destroy(r.Context()); err != nil {
		return err
	}
	sessions.Put(r.Context(), accountIDKey, accountID)
	return commitSession(w, r, sessions)
}
