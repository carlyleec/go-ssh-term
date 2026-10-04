package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
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

type LogoutInput struct {
	Origin      string `header:"Origin" required:"true" doc:"Configured browser origin"`
	ContentType string `header:"Content-Type" required:"true" doc:"application/json, optionally with media-type parameters"`
}

type LogoutOutput struct {
	SetCookie    string   `header:"Set-Cookie"`
	CacheControl []string `header:"Cache-Control"`
}

// RegisterLogout does not require an account: expired, anonymous, and
// deleted-account sessions must still be able to clear their browser cookie.
func (a *Access) RegisterLogout(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/auth/logout",
		Summary:       "End the current browser session",
		DefaultStatus: http.StatusNoContent,
		Responses:     authResponses(api, http.StatusForbidden, http.StatusUnsupportedMediaType, http.StatusServiceUnavailable),
		Middlewares: huma.Middlewares{humaMiddleware(func(next http.Handler) http.Handler {
			return authRequest(a.origin, loadSession(a.sessions, next))
		})},
	}, func(ctx context.Context, _ *LogoutInput) (*LogoutOutput, error) {
		session := a.loginSession(ctx)
		if err := a.sessions.Destroy(ctx); err != nil {
			return nil, &AuthErrorBody{Message: "could not sign out; try again", status: http.StatusServiceUnavailable}
		}
		// Revoke this owner before success, including attempts admitted before
		// session deletion but not yet published in the terminal registry.
		if session.ID != "" && a.OnLogout != nil {
			a.OnLogout(session)
		}
		cookie := http.Cookie{
			Name:        a.sessions.Cookie.Name,
			Domain:      a.sessions.Cookie.Domain,
			Path:        a.sessions.Cookie.Path,
			HttpOnly:    a.sessions.Cookie.HttpOnly,
			Secure:      a.sessions.Cookie.Secure,
			SameSite:    a.sessions.Cookie.SameSite,
			Partitioned: a.sessions.Cookie.Partitioned,
			Expires:     time.Unix(1, 0),
			MaxAge:      -1,
		}
		return &LogoutOutput{
			SetCookie:    cookie.String(),
			CacheControl: []string{`no-cache="Set-Cookie"`},
		}, nil
	})
}
