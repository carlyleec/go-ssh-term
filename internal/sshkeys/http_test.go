package sshkeys

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
	"github.com/carlyleec/go-ssh-term/internal/web"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

const keyTestOrigin = "http://localhost:5173"

type keyHTTPFixture struct {
	db         *sql.DB
	http       http.Handler
	encryption *Encryption
	access     *auth.Access
	sessions   *scs.SessionManager
	owners     [2]string
	cookies    [2]*http.Cookie
}

func newKeyHTTPFixture(t *testing.T) keyHTTPFixture {
	t.Helper()
	db, _ := testdb.New(t)
	sessions, stop := auth.NewSessions(config.Config{SessionLifetime: time.Hour}, db)
	t.Cleanup(stop)
	f := keyHTTPFixture{db: db, encryption: testEncryption(t), sessions: sessions}
	f.access = auth.NewAccess(sessions, db, "localhost", keyTestOrigin)
	f.http = keyTestHandler(db, f.encryption, f.access)
	for i := range f.owners {
		f.owners[i] = uuid.NewString()
		if err := queries.New(db).CreateAccount(t.Context(), queries.CreateAccountParams{
			ID: f.owners[i], DisplayName: "Account", RpID: "localhost", WebauthnUserHandle: []byte{byte(i)}, CreatedAt: 1,
		}); err != nil {
			t.Fatal(err)
		}
		f.cookies[i] = keySession(t, sessions, f.owners[i], false)
	}
	return f
}

func keySession(t *testing.T, sessions *scs.SessionManager, owner string, expired bool) *http.Cookie {
	t.Helper()
	ctx, err := sessions.Load(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	sessions.Put(ctx, "account_id", owner)
	if expired {
		sessions.SetDeadline(ctx, time.Now().Add(-time.Minute))
	}
	token, _, err := sessions.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: sessions.Cookie.Name, Value: token}
}

type uploadPart struct {
	name string
	data []byte
}

func keyUploadRequest(t *testing.T, parts ...uploadPart) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, part := range parts {
		var p io.Writer
		var err error
		if part.name == "private_key" {
			p, err = w.CreateFormFile(part.name, "demo_ed25519")
		} else {
			p, err = w.CreateFormField(part.name)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Write(part.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/keys", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}

func keyRequest(h http.Handler, r *http.Request, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func uploadKey(t *testing.T, f keyHTTPFixture, owner int, plain []byte) keyMetadata {
	t.Helper()
	w := keyRequest(f.http, keyUploadRequest(t, uploadPart{"name", []byte("  Demo key  ")}, uploadPart{"private_key", plain}), f.cookies[owner], keyTestOrigin)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload status = %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Key keyMetadata `json:"key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil || len(raw) != 1 || len(raw["key"]) != 4 {
		t.Fatal("upload returned unexpected metadata fields")
	}
	for _, field := range []string{"id", "name", "public_fingerprint", "created_at"} {
		if _, ok := raw["key"][field]; !ok {
			t.Fatal("missing metadata field")
		}
	}
	if body.Key.Name != "Demo key" || body.Key.CreatedAt.IsZero() {
		t.Fatal("invalid metadata")
	}
	if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Vary"), "Cookie") || len(w.Result().Cookies()) != 0 {
		t.Fatal("unexpected session/cache headers")
	}
	return body.Key
}

func TestKeyEndpointsOwnershipAndEncryptedStorage(t *testing.T) {
	f := newKeyHTTPFixture(t)
	plain := testUpload(t)
	first := uploadKey(t, f, 0, plain)
	var stored []byte
	var owner string
	var created int64
	if err := f.db.QueryRowContext(t.Context(), "SELECT account_id, encrypted_private_key, created_at FROM ssh_keys WHERE id = ?", first.ID).Scan(&owner, &stored, &created); err != nil {
		t.Fatal(err)
	}
	if owner != f.owners[0] || created != first.CreatedAt.UnixNano() || bytes.Contains(stored, plain) {
		t.Fatal("incorrect stored key metadata or plaintext persisted")
	}
	decrypted, err := f.encryption.decrypt(first.ID, owner, stored)
	defer clear(decrypted)
	if err != nil || !bytes.Equal(decrypted, plain) {
		t.Fatal("uploaded key cannot be decrypted")
	}
	fingerprint, err := Validate(decrypted)
	if err != nil || fingerprint != first.PublicFingerprint {
		t.Fatal("fingerprint does not identify uploaded key")
	}
	w := keyRequest(f.http, httptest.NewRequest("GET", "/api/keys", nil), f.cookies[1], "")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"keys":[]}` {
		t.Fatal("other account can list uploaded key")
	}
	w = keyRequest(f.http, httptest.NewRequest("DELETE", "/api/keys/"+first.ID, nil), f.cookies[1], keyTestOrigin)
	if w.Code != 404 {
		t.Fatal("other account can delete uploaded key")
	}
	second := uploadKey(t, f, 0, plain)
	third := uploadKey(t, f, 1, plain)
	if first.ID == second.ID || first.ID == third.ID {
		t.Fatal("uploads reused a record ID")
	}
	w = keyRequest(f.http, httptest.NewRequest("GET", "/api/keys", nil), f.cookies[0], "")
	var list struct {
		Keys []keyMetadata `json:"keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 || len(list.Keys) != 2 || list.Keys[0].ID != second.ID || list.Keys[1].ID != first.ID {
		t.Fatal("list order or ownership is incorrect")
	}
	var raw map[string][]map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil || len(raw) != 1 {
		t.Fatal("invalid list response")
	}
	for _, row := range raw["keys"] {
		if len(row) != 4 {
			t.Fatal("list exposed extra fields")
		}
	}
	if bytes.Contains(w.Body.Bytes(), stored) || strings.Contains(w.Body.String(), "PRIVATE KEY") {
		t.Fatal("list disclosed private material")
	}
	for _, method := range []string{"GET", "POST", "PUT"} {
		w = keyRequest(f.http, httptest.NewRequest(method, "/api/keys/"+first.ID, nil), f.cookies[0], keyTestOrigin)
		if w.Code != 405 {
			t.Fatalf("unexpected record endpoint: %s %d", method, w.Code)
		}
	}
	for i, want := range []int{204, 404} {
		w = keyRequest(f.http, httptest.NewRequest("DELETE", "/api/keys/"+first.ID, nil), f.cookies[0], keyTestOrigin)
		if w.Code != want {
			t.Fatalf("delete %d = %d", i, w.Code)
		}
	}
	var count int
	if err := f.db.QueryRowContext(t.Context(), "SELECT count(*) FROM ssh_keys").Scan(&count); err != nil || count != 2 {
		t.Fatal("delete affected unrelated keys")
	}
}

func TestKeyEndpointsRequireSessionAndOrigin(t *testing.T) {
	f := newKeyHTTPFixture(t)
	expired := keySession(t, f.sessions, f.owners[0], true)
	for _, method := range []string{"GET", "POST", "DELETE"} {
		path := "/api/keys"
		if method == "DELETE" {
			path += "/" + uuid.NewString()
		}
		for _, cookie := range []*http.Cookie{nil, expired} {
			w := keyRequest(f.http, httptest.NewRequest(method, path, nil), cookie, keyTestOrigin)
			if w.Code != 401 {
				t.Fatalf("unauthenticated %s = %d", method, w.Code)
			}
		}
		if method == "GET" {
			continue
		}
		for _, origin := range []string{"", "https://evil.example"} {
			w := keyRequest(f.http, httptest.NewRequest(method, path, nil), f.cookies[0], origin)
			if w.Code != 403 {
				t.Fatalf("untrusted origin %s = %d", method, w.Code)
			}
		}
	}
}

func TestKeyUploadValidation(t *testing.T) {
	f := newKeyHTTPFixture(t)
	valid := testUpload(t)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	protected, err := ssh.MarshalPrivateKeyWithPassphrase(key, "test", []byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	unsupportedKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	unsupported, err := ssh.MarshalPrivateKey(unsupportedKey, "test")
	if err != nil {
		t.Fatal(err)
	}
	name := uploadPart{"name", []byte("Demo")}
	file := uploadPart{"private_key", valid}
	for _, tc := range []struct {
		name  string
		parts []uploadPart
		want  int
	}{
		{"missing name", []uploadPart{file}, 400}, {"missing key", []uploadPart{name}, 400},
		{"duplicate name", []uploadPart{name, file, name}, 400}, {"duplicate key", []uploadPart{name, file, file}, 400},
		{"owner injection", []uploadPart{name, file, {"account_id", []byte(f.owners[1])}}, 400},
		{"blank name", []uploadPart{{"name", []byte(" \t")}, file}, 400},
		{"long name", []uploadPart{{"name", []byte(strings.Repeat("a", 65))}, file}, 400},
		{"control name", []uploadPart{{"name", []byte("a\nb")}, file}, 400},
		{"invalid UTF8", []uploadPart{{"name", []byte{255}}, file}, 400},
		{"empty key", []uploadPart{name, {"private_key", nil}}, 400},
		{"malformed key", []uploadPart{name, {"private_key", []byte("secret malformed input")}}, 400},
		{"protected key", []uploadPart{name, {"private_key", pem.EncodeToMemory(protected)}}, 400},
		{"unsupported algorithm", []uploadPart{name, {"private_key", pem.EncodeToMemory(unsupported)}}, 400},
		{"multiple keys", []uploadPart{name, {"private_key", append(bytes.Clone(valid), valid...)}}, 400},
		{"unsupported format", []uploadPart{name, {"private_key", []byte("ssh-ed25519 public-key-only")}}, 400},
		{"large key", []uploadPart{name, {"private_key", make([]byte, MaxUploadBytes+1)}}, 413},
		{"large request", []uploadPart{name, {"private_key", make([]byte, maxUploadRequestBytes)}}, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := keyRequest(f.http, keyUploadRequest(t, tc.parts...), f.cookies[0], keyTestOrigin)
			if w.Code != tc.want || strings.Contains(w.Body.String(), "secret") {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			var count int
			if err := f.db.QueryRowContext(t.Context(), "SELECT count(*) FROM ssh_keys").Scan(&count); err != nil || count != 0 {
				t.Fatal("invalid upload was persisted")
			}
		})
	}
	maximum := append(bytes.Clone(valid), bytes.Repeat([]byte(" "), MaxUploadBytes-len(valid))...)
	w := keyRequest(f.http, keyUploadRequest(t, uploadPart{"name", []byte(strings.Repeat("🐈", 64))}, uploadPart{"private_key", maximum}), f.cookies[0], keyTestOrigin)
	if w.Code != 201 {
		t.Fatalf("boundary upload failed: %d", w.Code)
	}
}

func TestKeyUploadBodyBounds(t *testing.T) {
	f := newKeyHTTPFixture(t)
	for _, kind := range []string{"content type", "mixed multipart", "truncated", "chunked file", "chunked epilogue", "chunked headers"} {
		r := keyUploadRequest(t, uploadPart{"name", []byte("Demo")}, uploadPart{"private_key", testUpload(t)})
		want := 400
		switch kind {
		case "content type":
			r.Header.Set("Content-Type", "application/json")
			want = 415
		case "mixed multipart":
			r.Header.Set("Content-Type", strings.Replace(r.Header.Get("Content-Type"), "multipart/form-data", "multipart/mixed", 1))
			want = 415
		case "truncated":
			typeHeader := r.Header.Get("Content-Type")
			body := new(bytes.Buffer)
			if _, err := body.ReadFrom(r.Body); err != nil {
				t.Fatal(err)
			}
			r = httptest.NewRequest("POST", "/api/keys", bytes.NewReader(body.Bytes()[:body.Len()-10]))
			r.Header.Set("Content-Type", typeHeader)
		case "chunked file":
			r = keyUploadRequest(t, uploadPart{"name", []byte("Demo")}, uploadPart{"private_key", make([]byte, MaxUploadBytes+1)})
			r.ContentLength = -1
			want = 413
		case "chunked epilogue":
			body := new(bytes.Buffer)
			body.ReadFrom(r.Body)
			body.Write(make([]byte, maxUploadRequestBytes))
			typeHeader := r.Header.Get("Content-Type")
			r = httptest.NewRequest("POST", "/api/keys", body)
			r.Header.Set("Content-Type", typeHeader)
			r.ContentLength = -1
			want = 413
		case "chunked headers":
			r = httptest.NewRequest("POST", "/api/keys", strings.NewReader("--boundary\r\nX-Large: "+strings.Repeat("a", maxUploadRequestBytes)+"\r\n\r\n"))
			r.ContentLength = -1
			r.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
			want = 413
		}
		w := keyRequest(f.http, r, f.cookies[0], keyTestOrigin)
		if w.Code != want {
			t.Fatalf("%s status = %d, want %d", kind, w.Code, want)
		}
	}
}

func TestKeyEndpointStorageFailures(t *testing.T) {
	for _, kind := range []string{"insert", "list", "delete", "encryption"} {
		t.Run(kind, func(t *testing.T) {
			f := newKeyHTTPFixture(t)
			plain := testUpload(t)
			var r *http.Request
			switch kind {
			case "insert", "encryption":
				r = keyUploadRequest(t, uploadPart{"name", []byte("Demo")}, uploadPart{"private_key", plain})
				if kind == "encryption" {
					f.http = keyTestHandler(f.db, nil, f.access)
				} else if _, err := f.db.ExecContext(t.Context(), "CREATE TRIGGER fail_insert BEFORE INSERT ON ssh_keys BEGIN SELECT RAISE(ABORT, 'secret storage details'); END"); err != nil {
					t.Fatal(err)
				}
			case "list":
				if _, err := f.db.ExecContext(t.Context(), "DROP TABLE ssh_keys"); err != nil {
					t.Fatal(err)
				}
				r = httptest.NewRequest("GET", "/api/keys", nil)
			case "delete":
				key := uploadKey(t, f, 0, plain)
				if _, err := f.db.ExecContext(t.Context(), "CREATE TRIGGER fail_delete BEFORE DELETE ON ssh_keys BEGIN SELECT RAISE(ABORT, 'secret storage details'); END"); err != nil {
					t.Fatal(err)
				}
				r = httptest.NewRequest("DELETE", "/api/keys/"+key.ID, nil)
			}
			w := keyRequest(f.http, r, f.cookies[0], keyTestOrigin)
			if w.Code != 503 || strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "PRIVATE KEY") {
				t.Fatal("failure was not reported safely")
			}
			if kind != "list" {
				var count int
				want := 0
				if kind == "delete" {
					want = 1
				}
				if err := f.db.QueryRowContext(t.Context(), "SELECT count(*) FROM ssh_keys").Scan(&count); err != nil || count != want {
					t.Fatal("failed operation changed stored keys")
				}
			}
		})
	}
}

func keyTestHandler(db *sql.DB, encryption *Encryption, access *auth.Access) http.Handler {
	mux := api.NewRouter()
	NewHandler(db, encryption).Register(api.New(mux), access)
	root := api.WithFrontend(mux, web.Handler(fstest.MapFS{}))
	return root
}

// These rejected requests must never be buffered or decoded by Huma.
type unreadUploadBody struct{ reads int }

func (b *unreadUploadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("body must not be read")
}
func (*unreadUploadBody) Close() error { return nil }

func TestUploadRejectsBeforeReadingBody(t *testing.T) {
	for _, kind := range []string{"anonymous", "origin", "oversized", "media-type"} {
		t.Run(kind, func(t *testing.T) {
			f := newKeyHTTPFixture(t)
			body := &unreadUploadBody{}
			request := httptest.NewRequest("POST", "/api/keys", nil)
			request.Body = body
			request.ContentLength = -1
			request.Header.Set("Content-Type", "multipart/form-data; boundary=example")
			cookie, origin, want := f.cookies[0], keyTestOrigin, 415
			switch kind {
			case "anonymous":
				cookie = nil
				want = 401
			case "origin":
				origin = "https://evil.example"
				want = 403
			case "oversized":
				request.ContentLength = maxUploadRequestBytes + 1
				want = 413
			case "media-type":
				request.Header.Set("Content-Type", "text/plain")
			}
			response := keyRequest(f.http, request, cookie, origin)
			if response.Code != want || body.reads != 0 {
				t.Fatalf("status=%d reads=%d: %s", response.Code, body.reads, response.Body.String())
			}
			var envelope map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || len(envelope) != 1 || envelope["error"] == "" {
				t.Fatalf("unsafe error envelope: %s", response.Body.String())
			}
		})
	}
}

func TestKeyTrafficMatchesContract(t *testing.T) {
	f := newKeyHTTPFixture(t)
	mux := api.NewRouter()
	contract := api.New(mux)
	NewHandler(f.db, f.encryption).Register(contract, f.access)
	check := func(operation *huma.Operation, response *httptest.ResponseRecorder, want int) {
		t.Helper()
		if response.Code != want {
			t.Fatalf("status=%d: %s", response.Code, response.Body.String())
		}
		documented := operation.Responses[strconv.Itoa(want)]
		if documented == nil {
			t.Fatalf("undocumented status %d", want)
		}
		if want == 204 {
			if response.Body.Len() != 0 || len(documented.Content) != 0 {
				t.Fatal("delete must have no content")
			}
			return
		}
		var value any
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		result := &huma.ValidateResult{}
		huma.Validate(contract.OpenAPI().Components.Schemas, documented.Content["application/json"].Schema, &huma.PathBuffer{}, huma.ModeReadFromServer, value, result)
		if len(result.Errors) != 0 {
			t.Fatalf("response does not match contract: %v", result.Errors)
		}
	}
	paths := contract.OpenAPI().Paths
	check(paths["/api/keys"].Get, keyRequest(mux, httptest.NewRequest("GET", "/api/keys", nil), f.cookies[0], keyTestOrigin), 200)
	upload := keyRequest(mux, keyUploadRequest(t, uploadPart{"name", []byte("Demo")}, uploadPart{"private_key", testUpload(t)}), f.cookies[0], keyTestOrigin)
	check(paths["/api/keys"].Post, upload, 201)
	var saved KeyBody
	if err := json.Unmarshal(upload.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	check(paths["/api/keys"].Get, keyRequest(mux, httptest.NewRequest("GET", "/api/keys", nil), f.cookies[0], keyTestOrigin), 200)
	for _, id := range []string{"not-a-uuid", "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"} {
		check(paths["/api/keys/{id}"].Delete, keyRequest(mux, httptest.NewRequest("DELETE", "/api/keys/"+id, nil), f.cookies[0], keyTestOrigin), 404)
	}
	check(paths["/api/keys/{id}"].Delete, keyRequest(mux, httptest.NewRequest("DELETE", "/api/keys/"+saved.Key.ID, nil), f.cookies[0], keyTestOrigin), 204)
	schema := paths["/api/keys"].Post.RequestBody.Content["multipart/form-data"].Schema
	schema = contract.OpenAPI().Components.Schemas.SchemaFromRef(schema.Ref)
	if len(schema.Properties) != 2 || len(schema.Required) != 2 || schema.Properties["private_key"].Format != "binary" {
		t.Fatal("incorrect multipart contract")
	}
	for _, operation := range []*huma.Operation{paths["/api/keys"].Get, paths["/api/keys"].Post, paths["/api/keys/{id}"].Delete} {
		if len(operation.Security) != 1 || operation.Security[0]["session"] == nil {
			t.Fatal("missing session security contract")
		}
	}
}
