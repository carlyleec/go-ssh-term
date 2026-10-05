package connections

import (
	"time"

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
