// Package routing declares the HTTP surface shared by serving and schema export.
package routing

import (
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/connections"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"github.com/danielgtaylor/huma/v2"
)

// Handlers holds already-constructed dependencies. Registration never calls them.
type Handlers struct {
	Access       *auth.Access
	Registration *auth.Registration
	Login        *auth.Login
	Keys         *sshkeys.Handler
	Connections  *connections.Handler
	Dialer       *connections.Dialer
}

func Register(contract huma.API, h Handlers, origin string) {
	access, registration, login := h.Access, h.Registration, h.Login
	keys, saved, hosts := h.Keys, h.Connections, h.Dialer
	auth.RegisterPasskeySchemas(contract)

	authAPI := huma.NewGroup(contract, "/api/auth")
	accountAPI := huma.NewGroup(authAPI)
	accountAPI.UseMiddleware(access.RequireHuma)
	huma.Get(accountAPI, "/me", access.GetCurrent, auth.Operation(contract, "currentUser"))

	// Logout must also clear anonymous and expired sessions.
	huma.Post(authAPI, "/logout", access.Logout, auth.Operation(contract, "logout"), api.WithMiddleware(access.LogoutMiddleware))
	huma.Post(authAPI, "/register/begin", registration.Begin, auth.Operation(contract, "beginRegistration"), api.WithMiddleware(registration.BeginMiddleware(origin)))
	huma.Post(authAPI, "/register/finish", registration.Finish, auth.Operation(contract, "finishRegistration"), api.WithMiddleware(registration.FinishMiddleware(origin)))
	huma.Post(authAPI, "/login/begin", login.Begin, auth.Operation(contract, "beginLogin"), api.WithMiddleware(login.BeginMiddleware(origin)))
	huma.Post(authAPI, "/login/finish", login.Finish, auth.Operation(contract, "finishLogin"), api.WithMiddleware(login.FinishMiddleware(origin)))

	protected := huma.NewGroup(contract, "/api")
	protected.UseMiddleware(access.RequireHuma)
	keysAPI := huma.NewGroup(protected, "/keys")
	huma.Get(keysAPI, "", keys.List, sshkeys.Operation(contract, "listKeys"))
	huma.Post(keysAPI, "", keys.Upload, sshkeys.Operation(contract, "uploadKey"))
	sshkeys.DocumentUpload(contract, contract.OpenAPI().Paths["/api/keys"].Post)
	huma.Delete(keysAPI, "/{id}", keys.Delete, sshkeys.Operation(contract, "deleteKey"))

	connectionsAPI := huma.NewGroup(protected, "/connections")
	jsonOnly := api.WithMiddleware(connections.RequireJSON(contract))
	huma.Post(connectionsAPI, "/import/preview", saved.PreviewImport, connections.Operation(contract, "previewConnectionImport"), jsonOnly)
	huma.Post(connectionsAPI, "/import/confirm", saved.ConfirmImport, connections.Operation(contract, "confirmConnectionImport"), jsonOnly)
	huma.Post(connectionsAPI, "", saved.Create, connections.Operation(contract, "createConnection"), jsonOnly)
	huma.Get(connectionsAPI, "", saved.List, connections.Operation(contract, "listConnections"))
	huma.Put(connectionsAPI, "/{id}", saved.Update, connections.Operation(contract, "updateConnection"), jsonOnly)
	huma.Delete(connectionsAPI, "/{id}", saved.Delete, connections.Operation(contract, "deleteConnection"))
	huma.Post(connectionsAPI, "/{id}/host-key", hosts.InspectHost, connections.Operation(contract, "inspectHost"))
	huma.Post(connectionsAPI, "/{id}/host-trust", hosts.ApproveHost, connections.Operation(contract, "approveHost"), jsonOnly)
	huma.Post(connectionsAPI, "/{id}/host-trust/reset", hosts.ResetHostTrust, connections.Operation(contract, "resetHostTrust"), jsonOnly)
}
