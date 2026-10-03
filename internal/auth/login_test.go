package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/queries"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func loginIdentity(t *testing.T) (loginUser, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	return loginUser{
		account:    queries.Account{ID: pgtype.UUID{Bytes: id, Valid: true}, DisplayName: "Same name", RpID: "localhost", WebauthnUserHandle: []byte(uuid.NewString())},
		credential: webauthn.Credential{ID: []byte(uuid.NewString()), PublicKey: public, AttestationType: "none", AttestationFormat: "none", Flags: webauthn.NewCredentialFlags(0x0d), Authenticator: webauthn.Authenticator{AAGUID: make([]byte, 16)}},
	}, key
}

func loginFixture(t *testing.T) (*login, http.Handler, loginUser, *ecdsa.PrivateKey) {
	t.Helper()
	reg, _ := registrationFixture(t)
	user, key := loginIdentity(t)
	h := &login{webauthn: reg.webauthn, sessions: reg.sessions, pending: make(map[string]webauthn.SessionData)}
	h.verify = func(_ context.Context, s webauthn.SessionData, p *protocol.ParsedCredentialAssertionData) (queries.Account, error) {
		if !bytes.Equal(p.RawID, user.credential.ID) || !bytes.Equal(p.Response.UserHandle, user.account.WebauthnUserHandle) {
			return queries.Account{}, errInvalidLogin
		}
		_, _, err := h.webauthn.ValidatePasskeyLogin(func([]byte, []byte) (webauthn.User, error) { return user, nil }, s, p)
		if err != nil {
			return queries.Account{}, errInvalidLogin
		}
		return user.account, nil
	}
	return h, h.routes(testOrigin), user, key
}

func loginRequestTest(handler http.Handler, path, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/auth/login/"+path, strings.NewReader(body))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func beginLoginTest(t *testing.T, handler http.Handler, cookie *http.Cookie) (protocol.CredentialAssertion, *http.Cookie) {
	t.Helper()
	w := loginRequestTest(handler, "begin", `{}`, testOrigin, cookie)
	if w.Code != 200 {
		t.Fatalf("begin: %d %s", w.Code, w.Body.String())
	}
	var options protocol.CredentialAssertion
	if err := json.Unmarshal(w.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if cookies := w.Result().Cookies(); len(cookies) > 0 {
		cookie = cookies[0]
	}
	if cookie == nil || options.Response.RelyingPartyID != "localhost" || options.Response.UserVerification != protocol.VerificationRequired || len(options.Response.AllowedCredentials) != 0 {
		t.Fatal("invalid discoverable login options")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("login options may be cached")
	}
	return options, cookie
}

func assertionResponse(t *testing.T, key *ecdsa.PrivateKey, user loginUser, options protocol.CredentialAssertion, count uint32, flags byte, origin, rp string) string {
	t.Helper()
	encode := base64.RawURLEncoding.EncodeToString
	client, err := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": encode(options.Response.Challenge), "origin": origin})
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(rp))
	data := append([]byte{}, hash[:]...)
	data = append(data, flags)
	data = binary.BigEndian.AppendUint32(data, count)
	clientHash := sha256.Sum256(client)
	signed := append(append([]byte{}, data...), clientHash[:]...)
	digest := sha256.Sum256(signed)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"id": encode(user.credential.ID), "rawId": encode(user.credential.ID), "type": "public-key", "response": map[string]any{"authenticatorData": encode(data), "clientDataJSON": encode(client), "signature": encode(signature), "userHandle": encode(user.account.WebauthnUserHandle)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestLoginSessionAndReplay(t *testing.T) {
	h, handler, user, key := loginFixture(t)
	options, cookie := beginLoginTest(t, handler, nil)
	ctx, err := h.sessions.Load(context.Background(), cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	h.sessions.Put(ctx, "anonymous_state", "discard")
	h.sessions.SetDeadline(ctx, time.Now().Add(time.Minute))
	if _, _, err := h.sessions.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	body := assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost")
	_, other := beginLoginTest(t, handler, nil)
	for _, c := range []*http.Cookie{nil, {Name: cookie.Name, Value: "invalid"}, other} {
		if w := loginRequestTest(handler, "finish", body, testOrigin, c); w.Code != 400 && w.Code != 401 {
			t.Fatal("another browser accepted")
		}
	}
	before := time.Now()
	w := loginRequestTest(handler, "finish", body, testOrigin, cookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), uuid.UUID(user.account.ID.Bytes).String()) {
		t.Fatalf("finish: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value == cookie.Value {
		t.Fatal("login did not replace session")
	}
	ctx, err = h.sessions.Load(context.Background(), cookies[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	if h.sessions.GetString(ctx, accountIDKey) != uuid.UUID(user.account.ID.Bytes).String() || h.sessions.Exists(ctx, loginBinding) || h.sessions.Exists(ctx, "anonymous_state") || h.sessions.Deadline(ctx).Before(before.Add(12*time.Hour)) {
		t.Fatal("new session retained anonymous state or deadline")
	}
	if _, found, err := h.sessions.Store.Find(cookie.Value); err != nil || found {
		t.Fatal("old session still exists")
	}
	if r := loginRequestTest(handler, "finish", body, testOrigin, cookie); r.Code != 400 {
		t.Fatal("login replay accepted")
	}
	for _, path := range []string{"begin", "finish"} {
		if r := loginRequestTest(handler, path, `{}`, testOrigin, cookies[0]); r.Code != 409 {
			t.Fatal("authenticated session can be overwritten")
		}
	}
}

func TestLoginInvalidAssertions(t *testing.T) {
	for _, failure := range []string{"signature", "origin", "rp", "challenge", "user handle", "credential ID", "user verification", "backup eligibility", "malformed", "oversized"} {
		t.Run(failure, func(t *testing.T) {
			_, handler, user, key := loginFixture(t)
			options, cookie := beginLoginTest(t, handler, nil)
			valid := assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost")
			origin, rp, flags := testOrigin, "localhost", byte(0x1d)
			switch failure {
			case "signature":
				_, key = loginIdentity(t)
			case "origin":
				origin = "http://evil.example"
			case "rp":
				rp = "other.example"
			case "challenge":
				options.Response.Challenge = []byte("wrong")
			case "user handle":
				user.account.WebauthnUserHandle = []byte("another-user")
			case "credential ID":
				user.credential.ID = []byte("unknown")
			case "user verification":
				flags = 0x19
			case "backup eligibility":
				flags = 0x05
			}
			body := assertionResponse(t, key, user, options, 1, flags, origin, rp)
			want := 401
			if failure == "malformed" {
				body = "{"
				want = 400
			}
			if failure == "oversized" {
				body = strings.Repeat(" ", 64*1024) + body
				want = 400
			}
			if w := loginRequestTest(handler, "finish", body, testOrigin, cookie); w.Code != want || len(w.Result().Cookies()) != 0 {
				t.Fatalf("bad assertion: %d %s", w.Code, w.Body.String())
			}
			if w := loginRequestTest(handler, "finish", valid, testOrigin, cookie); w.Code != 400 {
				t.Fatal("failed assertion allowed replay")
			}
		})
	}
}

func TestLoginPendingLifecycle(t *testing.T) {
	h, handler, user, key := loginFixture(t)
	first, cookie := beginLoginTest(t, handler, nil)
	second, _ := beginLoginTest(t, handler, cookie)
	if bytes.Equal(first.Response.Challenge, second.Response.Challenge) || len(h.pending) != 1 {
		t.Fatal("begin did not replace previous attempt")
	}
	if w := loginRequestTest(handler, "finish", assertionResponse(t, key, user, first, 1, 0x1d, testOrigin, "localhost"), testOrigin, cookie); w.Code != 401 {
		t.Fatal("replaced attempt accepted")
	}
	options, cookie := beginLoginTest(t, handler, cookie)
	for k, s := range h.pending {
		s.Expires = time.Now().Add(-time.Second)
		h.pending[k] = s
	}
	if w := loginRequestTest(handler, "finish", assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost"), testOrigin, cookie); w.Code != 400 {
		t.Fatal("expired attempt accepted")
	}
	options, cookie = beginLoginTest(t, handler, cookie)
	restarted := &login{webauthn: h.webauthn, sessions: h.sessions, verify: h.verify, pending: make(map[string]webauthn.SessionData)}
	if w := loginRequestTest(restarted.routes(testOrigin), "finish", assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost"), testOrigin, cookie); w.Code != 400 {
		t.Fatal("pending attempt survived restart")
	}
	h.pending = make(map[string]webauthn.SessionData)
	for i := range maxPendingLogins {
		h.pending[string(rune(i+1))] = webauthn.SessionData{Expires: time.Now().Add(time.Hour)}
	}
	if w := loginRequestTest(handler, "begin", `{}`, testOrigin, nil); w.Code != 503 {
		t.Fatal("pending store unbounded")
	}
	for k, s := range h.pending {
		s.Expires = time.Now().Add(-time.Second)
		h.pending[k] = s
	}
	beginLoginTest(t, handler, nil)
	if len(h.pending) != 1 {
		t.Fatal("expired attempts not pruned")
	}
}

func TestLoginConcurrentFinish(t *testing.T) {
	_, handler, user, key := loginFixture(t)
	options, cookie := beginLoginTest(t, handler, nil)
	body := assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost")
	start := make(chan struct{})
	results := make(chan int, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- loginRequestTest(handler, "finish", body, testOrigin, cookie).Code
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for code := range results {
		if code == 200 {
			successes++
		} else if code != 400 {
			t.Fatalf("unexpected status %d", code)
		}
	}
	if successes != 1 {
		t.Fatalf("successful finishes: %d", successes)
	}
}

func TestLoginRequestAndFailureHandling(t *testing.T) {
	h, handler, user, key := loginFixture(t)
	for _, body := range []string{"", `null`, `[]`, `{"display_name":"Alice"}`, `{} {}`, strings.Repeat(" ", 4096) + `{}`} {
		if w := loginRequestTest(handler, "begin", body, testOrigin, nil); w.Code != 400 {
			t.Fatal("invalid begin body accepted")
		}
	}
	for _, path := range []string{"begin", "finish"} {
		for _, origin := range []string{"", "null", "http://localhost:8080"} {
			if w := loginRequestTest(handler, path, `{}`, origin, nil); w.Code != 403 || len(w.Result().Cookies()) != 0 {
				t.Fatal("wrong origin accepted")
			}
		}
	}
	request := httptest.NewRequest("POST", "/api/auth/login/begin", strings.NewReader(`{}`))
	request.Header.Set("Origin", testOrigin)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 415 {
		t.Fatal("non-JSON request accepted")
	}
	request = httptest.NewRequest("GET", "/api/auth/login/begin", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 405 {
		t.Fatal("GET accepted")
	}
	options, cookie := beginLoginTest(t, handler, nil)
	h.verify = func(context.Context, webauthn.SessionData, *protocol.ParsedCredentialAssertionData) (queries.Account, error) {
		return queries.Account{}, errors.New("private database error")
	}
	body := assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost")
	if w := loginRequestTest(handler, "finish", body, testOrigin, cookie); w.Code != 503 || strings.Contains(w.Body.String(), "private") || len(w.Result().Cookies()) != 0 {
		t.Fatal("database failure granted access or leaked details")
	}
}

func TestLoginSessionStoreFailures(t *testing.T) {
	for _, operation := range []string{"begin commit", "load", "delete", "commit"} {
		t.Run(operation, func(t *testing.T) {
			h, handler, user, key := loginFixture(t)
			if operation == "begin commit" {
				h.sessions.Store = &failingSessionStore{Store: h.sessions.Store, failCommit: true}
				w := loginRequestTest(handler, "begin", `{}`, testOrigin, nil)
				if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "publicKey") {
					t.Fatal("begin returned options after session failure")
				}
				return
			}
			options, cookie := beginLoginTest(t, handler, nil)
			store := h.sessions.Store
			h.sessions.Store = &failingSessionStore{Store: store, failFind: operation == "load", failDelete: operation == "delete", failCommit: operation == "commit"}
			body := assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost")
			w := loginRequestTest(handler, "finish", body, testOrigin, cookie)
			h.sessions.Store = store
			if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), `"account":`) {
				t.Fatal("session failure returned successful login")
			}
			ctx, err := h.sessions.Load(context.Background(), cookie.Value)
			if err != nil || h.sessions.GetString(ctx, accountIDKey) != "" {
				t.Fatal("old cookie grants access after failure")
			}
			if operation != "load" {
				if retry := loginRequestTest(handler, "finish", body, testOrigin, cookie); retry.Code != 400 {
					t.Fatal("failed session allowed replay")
				}
			}
		})
	}
}
