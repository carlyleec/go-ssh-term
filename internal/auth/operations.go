package auth

import (
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/danielgtaylor/huma/v2"
)

// Operation supplies auth contract metadata; access rules belong to the route map.
func Operation(contract huma.API, id string) func(*huma.Operation) {
	op := huma.Operation{OperationID: id}
	switch id {
	case "currentUser":
		op.Summary = "Get the current account"
		op.Security = []map[string][]string{{"session": {}}}
		op.Responses = authResponses(contract, 401, 403, 503)
	case "logout":
		op.Summary = "End the current browser session"
		op.DefaultStatus = 204
		op.Responses = authResponses(contract, 403, 415, 503)
	case "beginRegistration", "beginLogin":
		op.MaxBodyBytes = 4096
		op.Responses = authResponses(contract, 400, 403, 409, 415, 500, 503)
		message := "invalid registration request"
		if id == "beginLogin" {
			message = "login begin expects an empty JSON object"
		}
		op.Metadata = map[string]any{"authBodyError": message}
	case "finishRegistration", "finishLogin":
		op.MaxBodyBytes = 64 * 1024
		// The WebAuthn parser owns credential validation, including extension data.
		op.SkipValidateBody = true
		op.DefaultStatus = 201
		op.Responses = authResponses(contract, 400, 403, 409, 415, 503)
		message := "passkey verification failed; begin again"
		if id == "finishLogin" {
			op.DefaultStatus = 200
			op.Responses = authResponses(contract, 400, 401, 403, 409, 415, 503)
			message = "invalid passkey response; begin again"
		}
		op.Metadata = map[string]any{"authBodyError": message}
	default:
		panic("unknown auth operation: " + id)
	}
	return api.Operation(op)
}
