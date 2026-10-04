package connections

import (
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/danielgtaylor/huma/v2"
)

type ConnectionFields struct {
	JumpConnectionID *string `json:"jump_connection_id,omitempty" nullable:"true" doc:"Optional owned direct connection UUID; omit or null for no jump"`
	Name             string  `json:"name" minLength:"1" maxLength:"256" doc:"Trimmed label, 1–64 Unicode characters without controls"`
	Host             string  `json:"host" minLength:"1" maxLength:"256" doc:"ASCII DNS hostname or unbracketed IPv4/IPv6 address; no port, zone, or URL"`
	Port             int64   `json:"port" minimum:"1" maximum:"65535"`
	Username         string  `json:"username" minLength:"1" maxLength:"128" doc:"Trimmed, 1–64 ASCII letters, digits, underscores, dots or hyphens; starts with a letter, digit or underscore"`
	SSHKeyID         string  `json:"ssh_key_id" doc:"Canonical UUID of an owned SSH key"`
}
type Connection struct {
	JumpConnectionID *string   `json:"jump_connection_id,omitempty" doc:"Owned direct connection UUID, when configured"`
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Host             string    `json:"host"`
	Port             int64     `json:"port"`
	Username         string    `json:"username"`
	SSHKeyID         string    `json:"ssh_key_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type ConnectionBody struct {
	Connection Connection `json:"connection"`
}
type ConnectionsBody struct {
	Connections []Connection `json:"connections" nullable:"false"`
}
type ConnectionOutput struct{ Body ConnectionBody }
type ListOutput struct{ Body ConnectionsBody }
type CreateInput struct{ Body ConnectionFields }
type UpdateInput struct {
	ID   string `path:"id"`
	Body ConnectionFields
}
type DeleteInput struct {
	ID string `path:"id"`
}

type ConnectionErrorBody struct {
	Message string `json:"error"`
	Hop     string `json:"hop,omitempty" doc:"bastion or target"`
	status  int
}

func (e *ConnectionErrorBody) Error() string  { return e.Message }
func (e *ConnectionErrorBody) GetStatus() int { return e.status }
func failure(status int, message string) *ConnectionErrorBody {
	return &ConnectionErrorBody{Message: message, status: status}
}

// Keep decoding failures safe and consistent without changing other API slices.
func init() {
	previous := huma.NewErrorWithContext
	huma.NewErrorWithContext = func(ctx huma.Context, status int, message string, details ...error) huma.StatusError {
		if ctx.Operation().Metadata["connectionErrors"] == true {
			if status == 422 {
				status = 400
			}
			return failure(status, "invalid connection request")
		}
		return previous(ctx, status, message, details...)
	}
}

func (h *handler) Register(api huma.API, access *auth.Access) {
	responses := func(statuses ...int) map[string]*huma.Response {
		schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[ConnectionErrorBody](), true, "")
		result := map[string]*huma.Response{}
		for _, status := range statuses {
			result[strconv.Itoa(status)] = &huma.Response{Description: http.StatusText(status), Content: map[string]*huma.MediaType{"application/json": {Schema: schema}}}
		}
		return result
	}
	operation := func(id, method, path, summary string, status int) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, DefaultStatus: status,
			Security: []map[string][]string{{"session": {}}}, Middlewares: huma.Middlewares{access.RequireHuma},
			Metadata: map[string]any{"connectionErrors": true}, Responses: responses(400, 401, 403, 404, 409, 413, 415, 503), MaxBodyBytes: 4096}
	}
	jsonOnly := func(ctx huma.Context, next func(huma.Context)) {
		media, _, err := mime.ParseMediaType(ctx.Header("Content-Type"))
		if err != nil || media != "application/json" {
			_ = huma.WriteErr(api, ctx, 415, "invalid connection request")
			return
		}
		next(ctx)
	}
	create := operation("createConnection", "POST", "/api/connections", "Create an owned saved connection", 201)
	create.Middlewares = append(create.Middlewares, jsonOnly)
	update := operation("updateConnection", "PUT", "/api/connections/{id}", "Replace an owned saved connection configuration", 200)
	update.Middlewares = append(update.Middlewares, jsonOnly)
	preview := operation("previewConnectionImport", "POST", "/api/connections/import/preview", "Preview a bounded SSH config without saving", 200)
	preview.MaxBodyBytes = 512 * 1024
	preview.Middlewares = append(preview.Middlewares, jsonOnly)
	huma.Register(api, preview, h.previewImport)
	confirm := operation("confirmConnectionImport", "POST", "/api/connections/import/confirm", "Revalidate and atomically save selected SSH config entries", 201)
	confirm.MaxBodyBytes = 512 * 1024
	confirm.Middlewares = append(confirm.Middlewares, jsonOnly)
	huma.Register(api, confirm, h.confirmImport)
	huma.Register(api, create, h.create)
	huma.Register(api, operation("listConnections", "GET", "/api/connections", "List owned saved connections", 200), h.list)
	huma.Register(api, update, h.update)
	huma.Register(api, operation("deleteConnection", "DELETE", "/api/connections/{id}", "Delete an owned saved connection", 204), h.delete)
}
