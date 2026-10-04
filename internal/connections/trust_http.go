package connections

import (
	"context"
	"mime"
	"net/http"
	"reflect"
	"strconv"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/danielgtaylor/huma/v2"
)

type HostOutput struct{ Body HostInspection }
type HostDecisionInput struct {
	ID   string `path:"id"`
	Body HostDecision
}

func (d *Dialer) Register(api huma.API, access *auth.Access) {
	op := func(id, path, summary string, status int) huma.Operation {
		schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[ConnectionErrorBody](), true, "")
		responses := map[string]*huma.Response{}
		for _, code := range []int{400, 401, 403, 404, 409, 413, 415, 502, 503, 504} {
			responses[strconv.Itoa(code)] = &huma.Response{Description: http.StatusText(code), Content: map[string]*huma.MediaType{"application/json": {Schema: schema}}}
		}
		return huma.Operation{OperationID: id, Method: "POST", Path: "/api/connections/{id}/" + path, Summary: summary, DefaultStatus: status, MaxBodyBytes: 4096, Security: []map[string][]string{{"session": {}}}, Middlewares: huma.Middlewares{access.RequireHuma}, Responses: responses, Metadata: map[string]any{"connectionErrors": true}}
	}
	jsonOnly := func(ctx huma.Context, next func(huma.Context)) {
		media, _, err := mime.ParseMediaType(ctx.Header("Content-Type"))
		if err != nil || media != "application/json" {
			_ = huma.WriteErr(api, ctx, 415, "invalid request")
			return
		}
		next(ctx)
	}
	huma.Register(api, op("inspectHost", "host-key", "Inspect an owned destination without authenticating", 200), func(ctx context.Context, input *DeleteInput) (*HostOutput, error) {
		account, _ := auth.AccountFromContext(ctx)
		result, err := d.Inspect(ctx, account.ID, input.ID)
		if err != nil {
			return nil, err
		}
		return &HostOutput{Body: result}, nil
	})
	approve := op("approveHost", "host-trust", "Approve the exact displayed host fingerprint", 200)
	approve.Middlewares = append(approve.Middlewares, jsonOnly)
	huma.Register(api, approve, func(ctx context.Context, input *HostDecisionInput) (*HostOutput, error) {
		account, _ := auth.AccountFromContext(ctx)
		result, err := d.Approve(ctx, account.ID, input.ID, input.Body)
		if err != nil {
			return nil, err
		}
		return &HostOutput{Body: result}, nil
	})
	reset := op("resetHostTrust", "host-trust/reset", "Explicitly remove the specified stored fingerprint", 204)
	reset.Middlewares = append(reset.Middlewares, jsonOnly)
	huma.Register(api, reset, func(ctx context.Context, input *HostDecisionInput) (*struct{}, error) {
		account, _ := auth.AccountFromContext(ctx)
		if err := d.ResetTrust(ctx, account.ID, input.ID, input.Body); err != nil {
			return nil, err
		}
		return &struct{}{}, nil
	})
}
