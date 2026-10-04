package sshkeys

import (
	"net/http"
	"reflect"
	"strconv"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

type KeyBody struct {
	Key keyMetadata `json:"key"`
}
type KeysBody struct {
	Keys []keyMetadata `json:"keys" nullable:"false"`
}
type UploadOutput struct{ Body KeyBody }
type ListOutput struct{ Body KeysBody }
type DeleteInput struct {
	ID string `path:"id"`
}

type KeyErrorBody struct {
	Message string `json:"error"`
	status  int
}

func (e *KeyErrorBody) Error() string  { return e.Message }
func (e *KeyErrorBody) GetStatus() int { return e.status }

// UploadForm describes the wire contract. readUpload streams and validates it;
// Huma's multipart decoder would allow private material to spill to disk.
type UploadForm struct {
	Name       string `json:"name" doc:"Trimmed label: 1–64 Unicode characters without controls; at most 256 raw UTF-8 bytes"`
	PrivateKey string `json:"private_key" format:"binary" doc:"Unencrypted Ed25519 OpenSSH private key, at most 16 KiB"`
}

type UploadInput struct {
	request *http.Request
	writer  http.ResponseWriter
}

func (input *UploadInput) Resolve(ctx huma.Context) []error {
	r, w := humago.Unwrap(ctx)
	input.request, input.writer = r.WithContext(ctx.Context()), w
	return nil
}

func (h *handler) Register(api huma.API, access *auth.Access) {
	responses := func(statuses ...int) map[string]*huma.Response {
		schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[KeyErrorBody](), true, "")
		result := map[string]*huma.Response{}
		for _, status := range statuses {
			result[strconv.Itoa(status)] = &huma.Response{Description: http.StatusText(status), Content: map[string]*huma.MediaType{"application/json": {Schema: schema}}}
		}
		return result
	}
	security := []map[string][]string{{"session": {}}}
	middleware := huma.Middlewares{access.RequireHuma}
	huma.Register(api, huma.Operation{
		OperationID: "listKeys", Method: http.MethodGet, Path: "/api/keys",
		Summary: "List owned SSH key metadata", Security: security, Middlewares: middleware,
		Responses: responses(401, 403, 503),
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "uploadKey", Method: http.MethodPost, Path: "/api/keys", DefaultStatus: http.StatusCreated,
		Summary: "Upload an SSH private key", Description: "Streams multipart parts with a 32 KiB total request limit. Duplicate, unknown, and transfer-encoded parts are rejected.",
		Security: security, Middlewares: middleware, Responses: responses(400, 401, 403, 413, 415, 503),
	}, h.upload)
	// Attach the multipart schema after handler registration. Huma treats an
	// object request schema as a request to buffer/decode even without a Body
	// field; this operation must leave every read to readUpload.
	api.OpenAPI().Paths["/api/keys"].Post.RequestBody = &huma.RequestBody{
		Required: true, Content: map[string]*huma.MediaType{
			"multipart/form-data": {Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[UploadForm](), true, "")},
		},
	}

	huma.Register(api, huma.Operation{
		OperationID: "deleteKey", Method: http.MethodDelete, Path: "/api/keys/{id}", DefaultStatus: http.StatusNoContent,
		Summary: "Delete an owned SSH key", Security: security, Middlewares: middleware,
		// The handler maps malformed IDs to the same 404 as missing or unowned keys.
		Responses: responses(401, 403, 404, 503),
	}, h.delete)
}
