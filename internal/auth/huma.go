package auth

import (
	"net/http"
	"reflect"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// AuthErrorBody preserves the error envelope used by the browser client.
type AuthErrorBody struct {
	Message string `json:"error"`
	status  int
}

func (e *AuthErrorBody) Error() string  { return e.Message }
func (e *AuthErrorBody) GetStatus() int { return e.status }

func authResponses(api huma.API, statuses ...int) map[string]*huma.Response {
	schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[AuthErrorBody](), true, "")
	responses := map[string]*huma.Response{}
	for _, status := range statuses {
		responses[strconv.Itoa(status)] = &huma.Response{
			Description: http.StatusText(status),
			Content:     map[string]*huma.MediaType{"application/json": {Schema: schema}},
		}
	}
	return responses
}

// Defer wrapping until a request arrives so contract export needs no services.
func humaMiddleware(wrap func(http.Handler) http.Handler) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, w := humago.Unwrap(ctx)
		wrap(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			// Session middleware supplies a new request context.
			next(huma.WithContext(ctx, r.Context()))
		})).ServeHTTP(w, r)
	}
}

// Huma's decoding failures must use the same safe envelope and 400 status as
// the existing ceremony handlers. Never serialize raw passkey data in errors.
func init() {
	previous := huma.NewErrorWithContext
	huma.NewErrorWithContext = func(ctx huma.Context, status int, message string, details ...error) huma.StatusError {
		if safe, ok := ctx.Operation().Metadata["authBodyError"].(string); ok {
			if status == http.StatusUnprocessableEntity || status == http.StatusRequestEntityTooLarge {
				status = http.StatusBadRequest
			}
			return &AuthErrorBody{Message: safe, status: status}
		}
		return previous(ctx, status, message, details...)
	}
}

// RequireHuma applies the same verified-account and origin checks to typed operations.
func (a *Access) RequireHuma(ctx huma.Context, next func(huma.Context)) {
	humaMiddleware(a.Require)(ctx, next)
}
