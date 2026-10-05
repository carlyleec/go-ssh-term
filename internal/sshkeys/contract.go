package sshkeys

import (
	"reflect"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/danielgtaylor/huma/v2"
)

func Operation(contract huma.API, id string) func(*huma.Operation) {
	op := huma.Operation{OperationID: id, Security: []map[string][]string{{"session": {}}}}
	switch id {
	case "listKeys":
		op.Summary = "List owned SSH key metadata"
		op.Responses = api.ErrorResponses[KeyErrorBody](contract, 401, 403, 503)
	case "uploadKey":
		op.Summary = "Upload an SSH private key"
		op.Description = "Streams multipart parts with a 32 KiB total request limit. Duplicate, unknown, and transfer-encoded parts are rejected."
		op.DefaultStatus = 201
		op.Responses = api.ErrorResponses[KeyErrorBody](contract, 400, 401, 403, 413, 415, 503)
	case "deleteKey":
		op.Summary = "Delete an owned SSH key"
		op.DefaultStatus = 204
		// Malformed IDs are indistinguishable from missing or unowned keys.
		op.Responses = api.ErrorResponses[KeyErrorBody](contract, 401, 403, 404, 409, 503)
	default:
		panic("unknown SSH key operation: " + id)
	}
	return api.Operation(op)
}

// DocumentUpload runs after registration: attaching the schema earlier would
// enable Huma body decoding and buffer private keys instead of streaming them.
func DocumentUpload(contract huma.API, operation *huma.Operation) {
	operation.RequestBody = &huma.RequestBody{
		Required: true, Content: map[string]*huma.MediaType{
			"multipart/form-data": {Schema: contract.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[UploadForm](), true, "")},
		},
	}
}
