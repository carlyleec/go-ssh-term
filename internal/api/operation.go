package api

import (
	"net/http"
	"reflect"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
)

// Operation keeps contract metadata explicit while the route map owns method/path.
func Operation(details huma.Operation) func(*huma.Operation) {
	return func(op *huma.Operation) {
		method, path := op.Method, op.Path
		*op = details
		op.Method, op.Path = method, path
	}
}

func WithMiddleware(middleware func(huma.Context, func(huma.Context))) func(*huma.Operation) {
	return func(op *huma.Operation) { op.Middlewares = append(op.Middlewares, middleware) }
}

func ErrorResponses[T any](contract huma.API, statuses ...int) map[string]*huma.Response {
	schema := contract.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[T](), true, "")
	responses := map[string]*huma.Response{}
	for _, status := range statuses {
		responses[strconv.Itoa(status)] = &huma.Response{
			Description: http.StatusText(status),
			Content:     map[string]*huma.MediaType{"application/json": {Schema: schema}},
		}
	}
	return responses
}
