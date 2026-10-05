package connections

import (
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"github.com/danielgtaylor/huma/v2"
)

func registerConnections(contract huma.API, saved *Handler, access *auth.Access) {
	protected := api.WithMiddleware(access.RequireHuma)
	jsonOnly := api.WithMiddleware(RequireJSON(contract))
	huma.Post(contract, "/api/connections/import/preview", saved.PreviewImport, Operation(contract, "previewConnectionImport"), protected, jsonOnly)
	huma.Post(contract, "/api/connections/import/confirm", saved.ConfirmImport, Operation(contract, "confirmConnectionImport"), protected, jsonOnly)
	huma.Post(contract, "/api/connections", saved.Create, Operation(contract, "createConnection"), protected, jsonOnly)
	huma.Get(contract, "/api/connections", saved.List, Operation(contract, "listConnections"), protected)
	huma.Put(contract, "/api/connections/{id}", saved.Update, Operation(contract, "updateConnection"), protected, jsonOnly)
	huma.Delete(contract, "/api/connections/{id}", saved.Delete, Operation(contract, "deleteConnection"), protected)
}

func registerTrust(contract huma.API, hosts *Dialer, access *auth.Access) {
	protected := api.WithMiddleware(access.RequireHuma)
	jsonOnly := api.WithMiddleware(RequireJSON(contract))
	huma.Post(contract, "/api/connections/{id}/host-key", hosts.InspectHost, Operation(contract, "inspectHost"), protected)
	huma.Post(contract, "/api/connections/{id}/host-trust", hosts.ApproveHost, Operation(contract, "approveHost"), protected, jsonOnly)
	huma.Post(contract, "/api/connections/{id}/host-trust/reset", hosts.ResetHostTrust, Operation(contract, "resetHostTrust"), protected, jsonOnly)
}

func registerKeys(contract huma.API, keys *sshkeys.Handler, access *auth.Access) {
	protected := api.WithMiddleware(access.RequireHuma)
	huma.Get(contract, "/api/keys", keys.List, sshkeys.Operation(contract, "listKeys"), protected)
	huma.Post(contract, "/api/keys", keys.Upload, sshkeys.Operation(contract, "uploadKey"), protected)
	sshkeys.DocumentUpload(contract, contract.OpenAPI().Paths["/api/keys"].Post)
	huma.Delete(contract, "/api/keys/{id}", keys.Delete, sshkeys.Operation(contract, "deleteKey"), protected)
}

func registerLogout(contract huma.API, access *auth.Access) {
	huma.Post(contract, "/api/auth/logout", access.Logout, auth.Operation(contract, "logout"), api.WithMiddleware(access.LogoutMiddleware))
}
