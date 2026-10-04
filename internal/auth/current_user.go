package auth

import (
	"context"
	"net/http"
	"reflect"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

type CurrentUserOutput struct {
	Body struct {
		Account Account `json:"account"`
	}
}

type AuthErrorBody struct {
	Error string `json:"error"`
}

// RegisterCurrentUser declares the operation without loading runtime services.
// A zero Access is sufficient for spec export; serving requests requires NewAccess.
func (a *Access) RegisterCurrentUser(api huma.API) {
	errorSchema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[AuthErrorBody](), true, "")
	responses := map[string]*huma.Response{}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable} {
		responses[strconv.Itoa(status)] = &huma.Response{
			Description: http.StatusText(status),
			Content: map[string]*huma.MediaType{
				"application/json": {Schema: errorSchema},
			},
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "currentUser",
		Method:      http.MethodGet,
		Path:        "/api/auth/me",
		Summary:     "Get the current account",
		Security:    []map[string][]string{{"session": {}}},
		Responses:   responses,
		Middlewares: huma.Middlewares{a.requireHuma},
	}, func(ctx context.Context, _ *struct{}) (*CurrentUserOutput, error) {
		output := &CurrentUserOutput{}
		output.Body.Account, _ = AccountFromContext(ctx)
		return output, nil
	})
}

func (a *Access) requireHuma(ctx huma.Context, next func(huma.Context)) {
	r, w := humago.Unwrap(ctx)
	a.Require(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Require loads the session into a new request context.
		next(huma.WithContext(ctx, r.Context()))
	})).ServeHTTP(w, r)
}
