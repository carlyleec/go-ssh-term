package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2/memstore"
	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const testOrigin = "http://localhost:5173"

func registrationFixture(t *testing.T) (*registration, http.Handler) {
	t.Helper()
	cfg := config.Config{BrowserOrigin: testOrigin, RPID: "localhost", SessionLifetime: 12 * time.Hour, ChallengeLifetime: 5 * time.Minute}
	wa, err := NewWebAuthn(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sessions, stop := NewSessions(cfg, nil)
	t.Cleanup(stop)
	sessions.Store = memstore.NewWithCleanupInterval(0)
	h := &registration{webauthn: wa, sessions: sessions, pending: make(map[string]pendingRegistration)}
	return h, h.routes(testOrigin)
}

func registrationRequestTest(handler http.Handler, path, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/auth/register/"+path, strings.NewReader(body))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func beginTest(t *testing.T, handler http.Handler, cookie *http.Cookie) (protocol.CredentialCreation, *http.Cookie) {
	t.Helper()
	w := registrationRequestTest(handler, "begin", `{"display_name":"  Alice 🐈  "}`, testOrigin, cookie)
	if w.Code != 200 {
		t.Fatalf("begin: %d %s", w.Code, w.Body.String())
	}
	var options protocol.CredentialCreation
	if err := json.Unmarshal(w.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) > 0 {
		cookie = cookies[0]
	}
	if cookie == nil || options.Response.User.DisplayName != "Alice 🐈" || options.Response.RelyingParty.ID != "localhost" {
		t.Fatal("invalid begin options or missing browser cookie")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("registration response may be cached")
	}
	return options, cookie
}

// Build an attestation=none response with real COSE public-key material so tests
// exercise the library's verification rather than replacing it with a stub.
func credentialResponse(t *testing.T, options protocol.CredentialCreation, origin, rp string, flags byte) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cose, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	id := []byte("test-credential-id")
	hash := sha256.Sum256([]byte(rp))
	data := append([]byte{}, hash[:]...)
	data = append(data, flags)
	data = append(data, make([]byte, 4+16)...)
	data = binary.BigEndian.AppendUint16(data, uint16(len(id)))
	data = append(data, id...)
	data = append(data, cose...)
	attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "authData": data, "attStmt": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	client, _ := json.Marshal(map[string]any{"type": "webauthn.create", "challenge": encode(options.Response.Challenge), "origin": origin})
	body, err := json.Marshal(map[string]any{"id": encode(id), "rawId": encode(id), "type": "public-key", "authenticatorAttachment": "platform", "response": map[string]any{"clientDataJSON": encode(client), "attestationObject": encode(attestation), "transports": []string{"internal"}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestRegistrationVerification(t *testing.T) {
	_, handler := registrationFixture(t)
	options, cookie := beginTest(t, handler, nil)
	body := credentialResponse(t, options, testOrigin, "localhost", 0x45)
	_, otherCookie := beginTest(t, handler, nil)
	for _, other := range []*http.Cookie{nil, {Name: cookie.Name, Value: "invalid"}, otherCookie} {
		w := registrationRequestTest(handler, "finish", body, testOrigin, other)
		if w.Code != 400 {
			t.Fatalf("unbound browser accepted: %d", w.Code)
		}
	}
	w := registrationRequestTest(handler, "finish", body, testOrigin, cookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"account_created":false`) || !strings.Contains(w.Body.String(), `"verified":true`) {
		t.Fatalf("finish: %d %s", w.Code, w.Body.String())
	}
	if w = registrationRequestTest(handler, "finish", body, testOrigin, cookie); w.Code != 400 {
		t.Fatal("challenge reused")
	}
}

func TestRegistrationFailuresConsumeChallenge(t *testing.T) {
	for _, failure := range []string{"origin", "rp", "verification", "challenge", "malformed", "oversized"} {
		t.Run(failure, func(t *testing.T) {
			_, handler := registrationFixture(t)
			options, cookie := beginTest(t, handler, nil)
			good := credentialResponse(t, options, testOrigin, "localhost", 0x45)
			bad := ""
			switch failure {
			case "origin":
				bad = credentialResponse(t, options, "http://evil.example", "localhost", 0x45)
			case "rp":
				bad = credentialResponse(t, options, testOrigin, "other.example", 0x45)
			case "verification":
				bad = credentialResponse(t, options, testOrigin, "localhost", 0x41)
			case "challenge":
				options.Response.Challenge = []byte("wrong")
				bad = credentialResponse(t, options, testOrigin, "localhost", 0x45)
			case "malformed":
				bad = "{"
			case "oversized":
				bad = strings.Repeat(" ", 64*1024) + good
			}
			if w := registrationRequestTest(handler, "finish", bad, testOrigin, cookie); w.Code != 400 {
				t.Fatalf("invalid response accepted: %d %s", w.Code, w.Body.String())
			}
			if w := registrationRequestTest(handler, "finish", good, testOrigin, cookie); w.Code != 400 {
				t.Fatal("failed attempt did not consume challenge")
			}
		})
	}
}

func TestRegistrationConcurrentFinish(t *testing.T) {
	_, handler := registrationFixture(t)
	options, cookie := beginTest(t, handler, nil)
	body := credentialResponse(t, options, testOrigin, "localhost", 0x45)
	start := make(chan struct{})
	results := make(chan int, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- registrationRequestTest(handler, "finish", body, testOrigin, cookie).Code
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for status := range results {
		if status == 200 {
			successes++
		} else if status != 400 {
			t.Fatalf("unexpected status %d", status)
		}
	}
	if successes != 1 {
		t.Fatalf("expected one successful finish, got %d", successes)
	}
}

func TestRegistrationExpiryReplacementAndCapacity(t *testing.T) {
	h, handler := registrationFixture(t)
	first, cookie := beginTest(t, handler, nil)
	second, _ := beginTest(t, handler, cookie)
	if bytes.Equal(first.Response.Challenge, second.Response.Challenge) || len(h.pending) != 1 {
		t.Fatal("begin must replace previous attempt")
	}
	if w := registrationRequestTest(handler, "finish", credentialResponse(t, first, testOrigin, "localhost", 0x45), testOrigin, cookie); w.Code != 400 {
		t.Fatal("replaced attempt accepted")
	}
	options, cookie := beginTest(t, handler, cookie)
	for key, value := range h.pending {
		value.session.Expires = time.Now().Add(-time.Second)
		h.pending[key] = value
	}
	if w := registrationRequestTest(handler, "finish", credentialResponse(t, options, testOrigin, "localhost", 0x45), testOrigin, cookie); w.Code != 400 {
		t.Fatal("expired attempt accepted")
	}
	for i := range maxPendingRegistrations {
		h.pending[string(rune(i+1))] = pendingRegistration{session: webauthn.SessionData{Expires: time.Now().Add(time.Hour)}}
	}
	if w := registrationRequestTest(handler, "begin", `{"display_name":"Alice"}`, testOrigin, nil); w.Code != 503 {
		t.Fatal("unbounded pending registrations")
	}
	for key, value := range h.pending {
		value.session.Expires = time.Now().Add(-time.Second)
		h.pending[key] = value
	}
	beginTest(t, handler, nil)
	if len(h.pending) != 1 {
		t.Fatal("expired registrations were not pruned")
	}
}

func TestRegistrationInputAndOrigin(t *testing.T) {
	_, handler := registrationFixture(t)
	for _, body := range []string{`{}`, `null`, `{"display_name":"  "}`, `{"display_name":"a\nb"}`, `{"display_name":1}`, `{"display_name":"Alice","extra":true}`, `{"display_name":"Alice"}{}`, `{"display_name":"` + strings.Repeat("a", 65) + `"}`, strings.Repeat(" ", 4096) + `{}`} {
		if w := registrationRequestTest(handler, "begin", body, testOrigin, nil); w.Code != 400 {
			t.Fatalf("invalid input accepted: %d", w.Code)
		}
	}
	for _, origin := range []string{"", "null", "http://localhost:8080", "http://evil.example"} {
		for _, path := range []string{"begin", "finish"} {
			if w := registrationRequestTest(handler, path, `{}`, origin, nil); w.Code != 403 || len(w.Result().Cookies()) != 0 {
				t.Fatalf("origin %q accepted", origin)
			}
		}
	}
	r := httptest.NewRequest("POST", "/api/auth/register/begin", strings.NewReader(`{}`))
	r.Header.Set("Origin", testOrigin)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatal("missing JSON media type accepted")
	}
	r = httptest.NewRequest("GET", "/api/auth/register/begin", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal("GET accepted")
	}
}

func TestRegistrationRestart(t *testing.T) {
	h, handler := registrationFixture(t)
	options, cookie := beginTest(t, handler, nil)
	restarted := NewRegistration(h.webauthn, h.sessions, testOrigin)
	body := credentialResponse(t, options, testOrigin, "localhost", 0x45)
	if w := registrationRequestTest(restarted, "finish", body, testOrigin, cookie); w.Code != 400 {
		t.Fatal("pending registration survived a new process-local store")
	}
	options, cookie = beginTest(t, restarted, cookie)
	body = credentialResponse(t, options, testOrigin, "localhost", 0x45)
	if w := registrationRequestTest(restarted, "finish", body, testOrigin, cookie); w.Code != 200 {
		t.Fatal("could not register again with the retained browser session")
	}
}
