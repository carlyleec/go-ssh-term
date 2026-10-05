package connections

import (
	"mime"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/danielgtaylor/huma/v2"
)

func Operation(contract huma.API, id string) func(*huma.Operation) {
	op := huma.Operation{
		OperationID: id, DefaultStatus: 200, MaxBodyBytes: 4096,
		Security:  []map[string][]string{{"session": {}}},
		Metadata:  map[string]any{"connectionErrors": true},
		Responses: api.ErrorResponses[ConnectionErrorBody](contract, 400, 401, 403, 404, 409, 413, 415, 503),
	}
	switch id {
	case "createConnection":
		op.Summary, op.DefaultStatus = "Create an owned saved connection", 201
	case "listConnections":
		op.Summary = "List owned saved connections"
	case "updateConnection":
		op.Summary = "Replace an owned saved connection configuration"
	case "deleteConnection":
		op.Summary, op.DefaultStatus = "Delete an owned saved connection", 204
	case "previewConnectionImport":
		op.Summary = "Preview a bounded SSH config without saving"
		op.MaxBodyBytes = 512 * 1024
	case "confirmConnectionImport":
		op.Summary, op.DefaultStatus = "Revalidate and atomically save selected SSH config entries", 201
		op.MaxBodyBytes = 512 * 1024
	case "inspectHost", "approveHost", "resetHostTrust":
		op.Responses = api.ErrorResponses[ConnectionErrorBody](contract, 400, 401, 403, 404, 409, 413, 415, 502, 503, 504)
		switch id {
		case "inspectHost":
			op.Summary = "Inspect the next host requiring verification on an owned route"
		case "approveHost":
			op.Summary = "Approve the exact displayed host fingerprint"
		case "resetHostTrust":
			op.Summary, op.DefaultStatus = "Explicitly remove the specified stored fingerprint", 204
		}
	default:
		panic("unknown connection operation: " + id)
	}
	return api.Operation(op)
}

func RequireJSON(contract huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		media, _, err := mime.ParseMediaType(ctx.Header("Content-Type"))
		if err != nil || media != "application/json" {
			_ = huma.WriteErr(contract, ctx, 415, "invalid connection request")
			return
		}
		next(ctx)
	}
}
