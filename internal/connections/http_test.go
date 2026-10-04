package connections

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"github.com/google/uuid"
)

const origin = "http://localhost:5173"

type fixture struct {
	db           *sql.DB
	handler      http.Handler
	sessions     *scs.SessionManager
	owners, keys [2]string
	cookies      [2]*http.Cookie
}

func setup(t *testing.T) fixture {
	t.Helper()
	db, _ := testdb.New(t)
	sessions, stop := auth.NewSessions(config.Config{SessionLifetime: time.Hour}, db)
	t.Cleanup(stop)
	mux := http.NewServeMux()
	contract := api.New(mux)
	access := auth.NewAccess(sessions, db, "localhost", origin)
	NewHandler(db).Register(contract, access)
	NewHandler(db).RegisterTerminal(mux, access, origin)
	sshkeys.NewHandler(db, nil).Register(contract, access)
	f := fixture{db: db, handler: mux, sessions: sessions}
	for i := range f.owners {
		f.owners[i], f.keys[i] = uuid.NewString(), uuid.NewString()
		if _, err := db.Exec(`INSERT INTO accounts VALUES (?, 'Owner', 'localhost', ?, 1)`, f.owners[i], []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO ssh_keys VALUES (?, ?, 'Key', 'SHA256:test', X'01ff', 1)`, f.keys[i], f.owners[i]); err != nil {
			t.Fatal(err)
		}
		ctx, err := sessions.Load(t.Context(), "")
		if err != nil {
			t.Fatal(err)
		}
		sessions.Put(ctx, "account_id", f.owners[i])
		token, _, err := sessions.Commit(ctx)
		if err != nil {
			t.Fatal(err)
		}
		f.cookies[i] = &http.Cookie{Name: sessions.Cookie.Name, Value: token}
	}
	return f
}
func (f fixture) fields() ConnectionFields {
	return ConnectionFields{Name: "  Lab  ", Host: " BASTION. ", Port: 22, Username: " demo ", SSHKeyID: f.keys[0]}
}
func (f fixture) request(method, path string, body any, owner int) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	r := httptest.NewRequest(method, path, reader)
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	if owner >= 0 {
		r.AddCookie(f.cookies[owner])
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func result(t *testing.T, w *httptest.ResponseRecorder, status int) Connection {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d, want %d: %s", w.Code, status, w.Body)
	}
	var body ConnectionBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Header().Get("Cache-Control") != "no-store" || len(w.Result().Cookies()) != 0 {
		t.Fatal("cache/session headers changed")
	}
	var fields map[string]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil || len(fields) != 1 || len(fields["connection"]) != 8 {
		t.Fatal("unexpected response fields")
	}
	return body.Connection
}
func expect(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d, want %d: %s", w.Code, status, w.Body)
	}
	if status >= 400 {
		var fields map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil || len(fields) != 1 || fields["error"] == "" {
			t.Fatalf("unsafe error: %s", w.Body)
		}
	}
}

func TestConnectionCRUDAndOwnership(t *testing.T) {
	f := setup(t)
	w := f.request("GET", "/api/connections", nil, 0)
	expect(t, w, 200)
	if strings.TrimSpace(w.Body.String()) != `{"connections":[]}` {
		t.Fatal("empty list is not an array")
	}
	first := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	if first.Name != "Lab" || first.Host != "bastion" || first.Username != "demo" || first.SSHKeyID != f.keys[0] || !first.CreatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("normalization: %+v", first)
	}
	second := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	w = f.request("GET", "/api/connections", nil, 0)
	var list ConnectionsBody
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Connections) != 2 || list.Connections[0].ID != second.ID {
		t.Fatal("list order/duplicate names")
	}
	if strings.TrimSpace(f.request("GET", "/api/connections", nil, 1).Body.String()) != `{"connections":[]}` {
		t.Fatal("cross-owner list")
	}
	fields := f.fields()
	fields.SSHKeyID = f.keys[1]
	for _, id := range []string{first.ID, "bad-id", uuid.NewString(), strings.ToUpper(first.ID)} {
		expect(t, f.request("PUT", "/api/connections/"+id, fields, 1), 404)
		expect(t, f.request("DELETE", "/api/connections/"+id, nil, 1), 404)
	}
	expect(t, f.request("POST", "/api/connections", fields, 0), 400)
	expect(t, f.request("PUT", "/api/connections/"+first.ID, fields, 0), 400)
	fields.SSHKeyID = uuid.NewString()
	expect(t, f.request("POST", "/api/connections", fields, 0), 400)
	expect(t, f.request("DELETE", "/api/keys/"+f.keys[0], nil, 0), 409)
	expect(t, f.request("DELETE", "/api/keys/"+f.keys[0], nil, 1), 404)
	fields = f.fields()
	fields.Name = "Edited"
	fields.Host = "2001:0db8::1"
	fields.Port = 2222
	edited := result(t, f.request("PUT", "/api/connections/"+first.ID, fields, 0), 200)
	if edited.ID != first.ID || edited.Host != "2001:db8::1" || edited.Name != "Edited" || !edited.CreatedAt.Equal(first.CreatedAt) || edited.UpdatedAt.Before(first.UpdatedAt) {
		t.Fatal("edit changed identity or failed to update")
	}
	for _, id := range []string{first.ID, second.ID} {
		w = f.request("DELETE", "/api/connections/"+id, nil, 0)
		expect(t, w, 204)
		if w.Body.Len() != 0 {
			t.Fatal("delete body")
		}
		expect(t, f.request("DELETE", "/api/connections/"+id, nil, 0), 404)
	}
	expect(t, f.request("DELETE", "/api/keys/"+f.keys[0], nil, 0), 204)
}

func TestConnectionValidationAndGuards(t *testing.T) {
	f := setup(t)
	for _, mutate := range []func(*ConnectionFields){
		func(v *ConnectionFields) { v.Name = " " }, func(v *ConnectionFields) { v.Name = strings.Repeat("猫", 65) }, func(v *ConnectionFields) { v.Name = "bad\x00name" },
		func(v *ConnectionFields) { v.Host = "https://host" }, func(v *ConnectionFields) { v.Host = "host:22" }, func(v *ConnectionFields) { v.Host = "[::1]" }, func(v *ConnectionFields) { v.Host = "fe80::1%eth0" },
		func(v *ConnectionFields) { v.Host = "bad..host" }, func(v *ConnectionFields) { v.Host = "-host" }, func(v *ConnectionFields) { v.Host = strings.Repeat("a", 64) }, func(v *ConnectionFields) { v.Host = "999.2.3.4" },
		func(v *ConnectionFields) { v.Port = 0 }, func(v *ConnectionFields) { v.Port = 65536 }, func(v *ConnectionFields) { v.Username = "-demo" }, func(v *ConnectionFields) { v.Username = "two words" },
		func(v *ConnectionFields) { v.SSHKeyID = "bad" },
	} {
		fields := f.fields()
		mutate(&fields)
		expect(t, f.request("POST", "/api/connections", fields, 0), 400)
	}
	fields := f.fields()
	fields.Name = strings.Repeat("猫", 64)
	fields.Host = "::ffff:192.0.2.1"
	fields.Port = 65535
	created := result(t, f.request("POST", "/api/connections", fields, 0), 201)
	if created.Host != "192.0.2.1" {
		t.Fatal("mapped IP not normalized")
	}
	for _, tc := range []struct {
		body, media string
		status      int
	}{
		{`{`, "application/json", 400}, {`null`, "application/json", 400}, {`{}`, "application/json", 400},
		{`{"account_id":"secret"}`, "application/json", 400},
		{strings.Repeat("x", 4097), "application/json", 413}, {`{}`, "text/plain", 415}, {`{}`, "", 415},
	} {
		r := httptest.NewRequest("POST", "/api/connections", strings.NewReader(tc.body))
		r.AddCookie(f.cookies[0])
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", tc.media)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		expect(t, w, tc.status)
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatal("reflected input")
		}
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		path := "/api/connections"
		if method == "PUT" || method == "DELETE" {
			path += "/" + created.ID
		}
		expect(t, f.request(method, path, f.fields(), -1), 401)
		if method != "GET" {
			r := httptest.NewRequest(method, path, &unreadBody{t: t})
			r.AddCookie(f.cookies[0])
			r.Header.Set("Origin", "https://wrong.example")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			expect(t, w, 403)
		}
	}
	// Anonymous requests must be rejected before decoding an unbounded body.
	r := httptest.NewRequest("POST", "/api/connections", &unreadBody{t: t})
	r.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	expect(t, w, 401)
}

type unreadBody struct{ t *testing.T }

func (b *unreadBody) Read([]byte) (int, error) { b.t.Error("unauthorized body read"); return 0, io.EOF }

func TestConnectionCreateRacesKeyDeletion(t *testing.T) {
	f := setup(t)
	var created, deleted *httptest.ResponseRecorder
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); created = f.request("POST", "/api/connections", f.fields(), 0) }()
	go func() { defer wg.Done(); deleted = f.request("DELETE", "/api/keys/"+f.keys[0], nil, 0) }()
	wg.Wait()
	if !(created.Code == 201 && deleted.Code == 409 || created.Code == 400 && deleted.Code == 204) {
		t.Fatalf("unsafe race outcome: create %d %s delete %d %s", created.Code, created.Body, deleted.Code, deleted.Body)
	}
	var dangling int
	if err := f.db.QueryRow(`SELECT count(*) FROM saved_connections c LEFT JOIN ssh_keys k ON c.ssh_key_id=k.id WHERE k.id IS NULL`).Scan(&dangling); err != nil || dangling != 0 {
		t.Fatalf("dangling reference: %d %v", dangling, err)
	}
}

func TestConnectionStorageFailures(t *testing.T) {
	f := setup(t)
	first := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	if _, err := f.db.Exec(`DROP TABLE saved_connections`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path string }{{"GET", "/api/connections"}, {"POST", "/api/connections"}, {"PUT", "/api/connections/" + first.ID}, {"DELETE", "/api/connections/" + first.ID}} {
		w := f.request(tc.method, tc.path, f.fields(), 0)
		expect(t, w, 503)
		if strings.Contains(w.Body.String(), "SQL") || strings.Contains(w.Body.String(), "saved_connections") {
			t.Fatal("storage details exposed")
		}
	}
}

func TestConnectionKeyReplacementAndExpiredSession(t *testing.T) {
	f := setup(t)
	first := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	replacement := uuid.NewString()
	if _, err := f.db.Exec(`INSERT INTO ssh_keys VALUES (?, ?, 'Replacement', 'SHA256:test', X'01ff', 1)`, replacement, f.owners[0]); err != nil {
		t.Fatal(err)
	}
	fields := f.fields()
	fields.SSHKeyID = replacement
	edited := result(t, f.request("PUT", "/api/connections/"+first.ID, fields, 0), 200)
	if edited.SSHKeyID != replacement {
		t.Fatal("key reference did not change")
	}
	expect(t, f.request("DELETE", "/api/keys/"+f.keys[0], nil, 0), 204)
	expect(t, f.request("DELETE", "/api/keys/"+replacement, nil, 0), 409)
	ctx, err := f.sessions.Load(t.Context(), f.cookies[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	f.sessions.SetDeadline(ctx, time.Now().Add(-time.Minute))
	if _, _, err := f.sessions.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		path := "/api/connections"
		if method == "PUT" || method == "DELETE" {
			path += "/" + first.ID
		}
		expect(t, f.request(method, path, fields, 0), 401)
	}
}
