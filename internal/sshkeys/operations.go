package sshkeys

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
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
	r, w := humachi.Unwrap(ctx)
	input.request, input.writer = r.WithContext(ctx.Context()), w
	return nil
}
