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
