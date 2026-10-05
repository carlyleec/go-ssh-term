package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-webauthn/webauthn/protocol"
)

func (h *registration) routes(origin string) http.Handler {
	mux := api.NewRouter()
	h.Register(api.New(mux), origin)
	return mux
}
func (h *login) routes(origin string) http.Handler {
	mux := api.NewRouter()
	h.Register(api.New(mux), origin)
	return mux
}

func TestPasskeyTrafficMatchesContract(t *testing.T) {
	reg, _ := registrationFixture(t)
	login, _, user, key := loginFixture(t)
	mux := api.NewRouter()
	contract := api.New(mux)
	reg.Register(contract, testOrigin)
	login.Register(contract, testOrigin)
	check := func(path string, body []byte, request bool, status int) {
		t.Helper()
		operation := contract.OpenAPI().Paths[path].Post
		schema := operation.RequestBody.Content["application/json"].Schema
		mode := huma.ModeWriteToServer
		if !request {
			schema = operation.Responses[strconv.Itoa(status)].Content["application/json"].Schema
			mode = huma.ModeReadFromServer
		}
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatal(err)
		}
		result := &huma.ValidateResult{}
		huma.Validate(contract.OpenAPI().Components.Schemas, schema, &huma.PathBuffer{}, mode, value, result)
		if len(result.Errors) > 0 {
			t.Fatalf("%s request=%v: %v", path, request, result.Errors)
		}
	}
	rb := registrationRequestTest(mux, "begin", `{"display_name":"Alice"}`, testOrigin, nil)
	if rb.Code != 200 {
		t.Fatalf("registration begin: %d %s", rb.Code, rb.Body.String())
	}
	check("/api/auth/register/begin", rb.Body.Bytes(), false, 200)
	var creation protocol.CredentialCreation
	if err := json.Unmarshal(rb.Body.Bytes(), &creation); err != nil {
		t.Fatal(err)
	}
	registrationBody := credentialResponse(t, creation, testOrigin, "localhost", 0x45)
	check("/api/auth/register/finish", []byte(registrationBody), true, 0)
	rf := registrationRequestTest(mux, "finish", registrationBody, testOrigin, rb.Result().Cookies()[0])
	if rf.Code != 201 {
		t.Fatalf("registration finish: %d %s", rf.Code, rf.Body.String())
	}
	check("/api/auth/register/finish", rf.Body.Bytes(), false, 201)
	lb := loginRequestTest(mux, "begin", `{}`, testOrigin, nil)
	if lb.Code != 200 {
		t.Fatalf("login begin: %d %s", lb.Code, lb.Body.String())
	}
	check("/api/auth/login/begin", lb.Body.Bytes(), false, 200)
	var assertion protocol.CredentialAssertion
	if err := json.Unmarshal(lb.Body.Bytes(), &assertion); err != nil {
		t.Fatal(err)
	}
	loginBody := assertionResponse(t, key, user, assertion, 1, 0x1d, testOrigin, "localhost")
	check("/api/auth/login/finish", []byte(loginBody), true, 0)
	lf := loginRequestTest(mux, "finish", loginBody, testOrigin, lb.Result().Cookies()[0])
	if lf.Code != 200 {
		t.Fatalf("login finish: %d %s", lf.Code, lf.Body.String())
	}
	check("/api/auth/login/finish", lf.Body.Bytes(), false, 200)
}

func TestMalformedPasskeyBodiesConsumeChallenges(t *testing.T) {
	for _, body := range []string{`{"secret":"do-not-echo"`, strings.Repeat(" ", 64*1024) + `{}`, `null`} {
		t.Run(strconv.Itoa(len(body)), func(t *testing.T) {
			_, handler := registrationFixture(t)
			options, cookie := beginTest(t, handler, nil)
			bad := registrationRequestTest(handler, "finish", body, testOrigin, cookie)
			assertSafePasskeyError(t, bad, 400)
			replay := registrationRequestTest(handler, "finish", credentialResponse(t, options, testOrigin, "localhost", 0x45), testOrigin, cookie)
			assertSafePasskeyError(t, replay, 400)
			if !strings.Contains(replay.Body.String(), "missing or expired") {
				t.Fatal("registration challenge was not consumed")
			}
			_, loginHandler, user, key := loginFixture(t)
			assertion, loginCookie := beginLoginTest(t, loginHandler, nil)
			bad = loginRequestTest(loginHandler, "finish", body, testOrigin, loginCookie)
			assertSafePasskeyError(t, bad, 400)
			replay = loginRequestTest(loginHandler, "finish", assertionResponse(t, key, user, assertion, 1, 0x1d, testOrigin, "localhost"), testOrigin, loginCookie)
			assertSafePasskeyError(t, replay, 400)
			if !strings.Contains(replay.Body.String(), "missing or expired") {
				t.Fatal("login challenge was not consumed")
			}
		})
	}
}

func TestRegistrationBeginStoreFailures(t *testing.T) {
	for _, operation := range []string{"load", "commit"} {
		t.Run(operation, func(t *testing.T) {
			h, handler := registrationFixture(t)
			cookie := accessCookie(t, h.sessions, "", false)
			h.sessions.Store = &failingSessionStore{Store: h.sessions.Store, failFind: operation == "load", failCommit: operation == "commit"}
			response := registrationRequestTest(handler, "begin", `{"display_name":"Alice"}`, testOrigin, cookie)
			assertSafePasskeyError(t, response, 503)
		})
	}
}

func assertSafePasskeyError(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" || len(w.Result().Cookies()) != 0 {
		t.Fatalf("response: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	message, ok := body["error"].(string)
	if len(body) != 1 || !ok || message == "" || strings.Contains(message, "do-not-echo") {
		t.Fatalf("unsafe envelope: %s", w.Body.String())
	}
}
