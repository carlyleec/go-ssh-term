package auth

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type CurrentUserOutput struct {
	Body struct {
		Account Account `json:"account"`
	}
}

// RegisterCurrentUser declares the operation without loading runtime services.
// A zero Access is sufficient for spec export; serving requests requires NewAccess.
func (a *Access) RegisterCurrentUser(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "currentUser",
		Method:      http.MethodGet,
		Path:        "/api/auth/me",
		Summary:     "Get the current account",
		Security:    []map[string][]string{{"session": {}}},
		Responses:   authResponses(api, http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable),
		Middlewares: huma.Middlewares{humaMiddleware(a.Require)},
	}, func(ctx context.Context, _ *struct{}) (*CurrentUserOutput, error) {
		output := &CurrentUserOutput{}
		output.Body.Account, _ = AccountFromContext(ctx)
		return output, nil
	})
}
