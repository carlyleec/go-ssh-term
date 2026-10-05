package routing

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/carlyleec/go-ssh-term/internal/connections"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func uploadBody(t *testing.T, key []byte, duplicate bool) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("name", "Demo key"); err != nil {
		t.Fatal(err)
	}
	if duplicate {
		if err := writer.WriteField("name", "Duplicate"); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("private_key", "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(key); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func privateKey(t *testing.T) ([]byte, ed25519.PublicKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(private, "test")
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(block), public
}

type countedBody struct {
	reader io.Reader
	bytes  int
}

func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.bytes += n
	return n, err
}
func (*countedBody) Close() error { return nil }

func TestSharedStreamingUpload(t *testing.T) {
	f := newRoutingFixture(t)
	plain, public := privateKey(t)
	body, media := uploadBody(t, plain, false)
	r := f.request("POST", "/api/keys", bytes.NewReader(body), f.cookie)
	r.Header.Set("Content-Type", media)
	r.ContentLength = -1
	w := f.serve(r)
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var saved sshkeys.KeyBody
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	signer, err := f.encryption.Signer(t.Context(), f.db, f.owner, saved.Key.ID)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signer.PublicKey().Marshal(), expected.Marshal()) {
		t.Fatal("stored key differs from upload")
	}
	var encrypted []byte
	if err := f.db.QueryRow("SELECT encrypted_private_key FROM ssh_keys WHERE id = ?", saved.Key.ID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted, plain) || bytes.Contains(w.Body.Bytes(), plain) {
		t.Fatal("private material exposed")
	}

	for _, kind := range []string{"duplicate", "key-limit", "request-limit", "chunked-limit", "media-type"} {
		t.Run(kind, func(t *testing.T) {
			payload := plain
			if kind == "key-limit" {
				payload = bytes.Repeat([]byte("x"), 16*1024+1)
			}
			body, media := uploadBody(t, payload, kind == "duplicate")
			if kind == "request-limit" || kind == "chunked-limit" {
				body = append(body, bytes.Repeat([]byte("x"), 33*1024)...)
			}
			r := f.request("POST", "/api/keys", bytes.NewReader(body), f.cookie)
			r.Header.Set("Content-Type", media)
			counted := &countedBody{reader: bytes.NewReader(body)}
			r.Body = counted
			want := 413
			switch kind {
			case "duplicate":
				want = 400
			case "chunked-limit":
				r.ContentLength = -1
			case "media-type":
				r.Header.Set("Content-Type", "application/json")
				want = 415
			}
			w := f.serve(r)
			if w.Code != want {
				t.Fatalf("got %d, want %d: %s", w.Code, want, w.Body)
			}
			if counted.bytes > 32*1024+1 {
				t.Fatalf("read past upload bound: %d", counted.bytes)
			}
			if (kind == "request-limit" || kind == "media-type") && counted.bytes != 0 {
				t.Fatalf("read rejected body: %d", counted.bytes)
			}
			var count int
			if err := f.db.QueryRow("SELECT count(*) FROM ssh_keys").Scan(&count); err != nil || count != 1 {
				t.Fatalf("rejected upload wrote data: %d, %v", count, err)
			}
		})
	}
}

func TestSharedRoutesEnforceResourceOwnership(t *testing.T) {
	f := newRoutingFixture(t)
	plain, _ := privateKey(t)
	body, media := uploadBody(t, plain, false)
	upload := f.request("POST", "/api/keys", bytes.NewReader(body), f.cookie)
	upload.Header.Set("Content-Type", media)
	w := f.serve(upload)
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var key sshkeys.KeyBody
	if err := json.Unmarshal(w.Body.Bytes(), &key); err != nil {
		t.Fatal(err)
	}
	fields := connections.ConnectionFields{Name: "Private host", Host: "127.0.0.1", Port: 1, Username: "demo", SSHKeyID: key.Key.ID}
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	w = f.serve(f.request("POST", "/api/connections", bytes.NewReader(data), f.cookie))
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var saved connections.ConnectionBody
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	foreignID := uuid.NewString()
	if _, err := f.db.Exec("INSERT INTO accounts VALUES (?, 'Other', 'localhost', X'02', 1)", foreignID); err != nil {
		t.Fatal(err)
	}
	ctx, err := f.sessions.Load(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	f.sessions.Put(ctx, "account_id", foreignID)
	token, _, err := f.sessions.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: f.cookie.Name, Value: token}
	prefix := "/api/connections/" + saved.Connection.ID
	decision := `{"host":"127.0.0.1","port":1,"fingerprint":"SHA256:test"}`
	for _, tc := range []struct{ method, path, body string }{
		{"DELETE", "/api/keys/" + key.Key.ID, ""},
		{"PUT", prefix, string(data)}, {"DELETE", prefix, ""},
		{"POST", prefix + "/host-key", `{}`}, {"POST", prefix + "/host-trust", decision}, {"POST", prefix + "/host-trust/reset", decision},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := f.serve(f.request(tc.method, tc.path, strings.NewReader(tc.body), cookie))
			if w.Code != 404 {
				t.Fatalf("foreign resource: %d %s", w.Code, w.Body)
			}
		})
	}
	for _, path := range []string{"/api/keys", "/api/connections"} {
		w := f.serve(f.request("GET", path, nil, cookie))
		if w.Code != 200 || strings.Contains(w.Body.String(), key.Key.ID) || strings.Contains(w.Body.String(), saved.Connection.ID) {
			t.Fatalf("list leaked another account's data: %d %s", w.Code, w.Body)
		}
	}
	w = f.serve(f.request("POST", "/api/connections", bytes.NewReader(data), cookie))
	if w.Code != 400 {
		t.Fatalf("foreign key selection: %d %s", w.Code, w.Body)
	}
	var count int
	if err := f.db.QueryRow("SELECT count(*) FROM saved_connections WHERE id = ? AND account_id = ?", saved.Connection.ID, f.owner).Scan(&count); err != nil || count != 1 {
		t.Fatalf("foreign mutation changed saved connection: %d %v", count, err)
	}
}
