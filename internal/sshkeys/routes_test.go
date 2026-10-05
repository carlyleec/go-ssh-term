package sshkeys

import (
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/danielgtaylor/huma/v2"
)

func registerKeys(contract huma.API, keys *Handler, access *auth.Access) {
	protected := api.WithMiddleware(access.RequireHuma)
	huma.Get(contract, "/api/keys", keys.List, Operation(contract, "listKeys"), protected)
	huma.Post(contract, "/api/keys", keys.Upload, Operation(contract, "uploadKey"), protected)
	DocumentUpload(contract, contract.OpenAPI().Paths["/api/keys"].Post)
	huma.Delete(contract, "/api/keys/{id}", keys.Delete, Operation(contract, "deleteKey"), protected)
}
