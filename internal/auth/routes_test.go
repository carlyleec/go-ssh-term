package auth

import (
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/danielgtaylor/huma/v2"
)

// Package-local fixtures exercise actions with their production options without
// importing the composition package, which itself depends on auth.
func registerCurrentUser(contract huma.API, access *Access) {
	huma.Get(contract, "/api/auth/me", access.GetCurrent, Operation(contract, "currentUser"), api.WithMiddleware(access.RequireHuma))
}

func registerLogout(contract huma.API, access *Access) {
	huma.Post(contract, "/api/auth/logout", access.Logout, Operation(contract, "logout"), api.WithMiddleware(access.LogoutMiddleware))
}

func registerRegistration(contract huma.API, h *Registration, origin string) {
	RegisterPasskeySchemas(contract)
	huma.Post(contract, "/api/auth/register/begin", h.Begin, Operation(contract, "beginRegistration"), api.WithMiddleware(h.BeginMiddleware(origin)))
	huma.Post(contract, "/api/auth/register/finish", h.Finish, Operation(contract, "finishRegistration"), api.WithMiddleware(h.FinishMiddleware(origin)))
}

func registerLogin(contract huma.API, h *Login, origin string) {
	RegisterPasskeySchemas(contract)
	huma.Post(contract, "/api/auth/login/begin", h.Begin, Operation(contract, "beginLogin"), api.WithMiddleware(h.BeginMiddleware(origin)))
	huma.Post(contract, "/api/auth/login/finish", h.Finish, Operation(contract, "finishLogin"), api.WithMiddleware(h.FinishMiddleware(origin)))
}
