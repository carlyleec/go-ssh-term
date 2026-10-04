package auth

import (
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/go-webauthn/webauthn/protocol"
)

// SessionRequest gives SCS access to the native request and cookie writer after
// Huma has decoded the body. Session state comes from the middleware context.
type SessionRequest struct {
	Origin      string `header:"Origin" required:"true" doc:"Configured browser origin"`
	ContentType string `header:"Content-Type" required:"true" doc:"application/json, optionally with media-type parameters"`
	request     *http.Request
	writer      http.ResponseWriter
}

func (s *SessionRequest) Resolve(ctx huma.Context) []error {
	r, w := humago.Unwrap(ctx)
	s.request, s.writer = r.WithContext(ctx.Context()), w
	return nil
}

type AccountBody struct {
	Account Account `json:"account"`
}
type AccountOutput struct{ Body AccountBody }

// User handles are always encoded bytes in this application. The upstream
// UserEntity allows any value, which is broader than the response we send.
type PasskeyUser struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

// These optional browser convenience fields are not needed to verify the
// attestation. Upstream's JSON tags do not express their optionality.
type PasskeyAttestation struct {
	ClientDataJSON     string   `json:"clientDataJSON"`
	AttestationObject  string   `json:"attestationObject"`
	Transports         []string `json:"transports,omitempty"`
	AuthenticatorData  string   `json:"authenticatorData,omitempty"`
	PublicKey          string   `json:"publicKey,omitempty"`
	PublicKeyAlgorithm int64    `json:"publicKeyAlgorithm,omitempty"`
}

func registerPasskeySchemas(api huma.API) {
	schemas := api.OpenAPI().Components.Schemas
	schemas.RegisterTypeAlias(reflect.TypeFor[protocol.URLEncodedBase64](), reflect.TypeFor[string]())
	schemas.RegisterTypeAlias(reflect.TypeFor[protocol.UserEntity](), reflect.TypeFor[PasskeyUser]())
	schemas.RegisterTypeAlias(reflect.TypeFor[protocol.AuthenticatorAttestationResponse](), reflect.TypeFor[PasskeyAttestation]())
}
