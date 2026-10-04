package connections

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/carlyleec/go-ssh-term/db/migrations"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/google/uuid"
)

func TestJumpConnectionsCRUDAndValidation(t *testing.T) {
	f := setup(t)
	a := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	b := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	fields := f.fields()
	fields.JumpConnectionID = &a.ID
	target := result(t, f.request("POST", "/api/connections", fields, 0), 201)
	if target.JumpConnectionID == nil || *target.JumpConnectionID != a.ID {
		t.Fatal("jump not returned")
	}
	if !strings.Contains(f.request("GET", "/api/connections", nil, 0).Body.String(), `"jump_connection_id":"`+a.ID+`"`) {
		t.Fatal("jump missing from list")
	}
	foreign := f.fields()
	foreign.SSHKeyID = f.keys[1]
	other := result(t, f.request("POST", "/api/connections", foreign, 1), 201)
	for _, id := range []string{"", "bad", uuid.NewString(), other.ID, target.ID} {
		fields.JumpConnectionID = &id
		expect(t, f.request("PUT", "/api/connections/"+target.ID, fields, 0), 400)
	}
	// A parent cannot acquire a jump while anything depends on it.
	fields.JumpConnectionID = &b.ID
	expect(t, f.request("PUT", "/api/connections/"+a.ID, fields, 0), 400)
	fields.JumpConnectionID = &target.ID
	expect(t, f.request("POST", "/api/connections", fields, 0), 400)
	expect(t, f.request("PUT", "/api/connections/"+a.ID, fields, 0), 400)
	expect(t, f.request("DELETE", "/api/connections/"+a.ID, nil, 0), 409)
	expect(t, f.request("DELETE", "/api/connections/"+a.ID, nil, 1), 404)
	fields.JumpConnectionID = &b.ID
	changed := result(t, f.request("PUT", "/api/connections/"+target.ID, fields, 0), 200)
	if changed.JumpConnectionID == nil || *changed.JumpConnectionID != b.ID {
		t.Fatal("jump replacement failed")
	}
	expect(t, f.request("DELETE", "/api/connections/"+a.ID, nil, 0), 204)
	fields.JumpConnectionID = nil
	direct := result(t, f.request("PUT", "/api/connections/"+target.ID, fields, 0), 200)
	if direct.JumpConnectionID != nil {
		t.Fatal("omission did not clear jump")
	}
	fields.JumpConnectionID = &b.ID
	result(t, f.request("PUT", "/api/connections/"+target.ID, fields, 0), 200)
	encoded, _ := json.Marshal(fields)
	var explicitNull map[string]any
	_ = json.Unmarshal(encoded, &explicitNull)
	explicitNull["jump_connection_id"] = nil
	cleared := result(t, f.request("PUT", "/api/connections/"+target.ID, explicitNull, 0), 200)
	if cleared.JumpConnectionID != nil {
		t.Fatal("null did not clear jump")
	}
	expect(t, f.request("DELETE", "/api/connections/"+b.ID, nil, 0), 204)
}

func TestJumpConcurrentEditsCannotCreateChain(t *testing.T) {
	f := setup(t)
	a := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	b := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	c := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	first, second := f.fields(), f.fields()
	first.JumpConnectionID = &b.ID
	second.JumpConnectionID = &c.ID
	gate := make(chan struct{})
	var work sync.WaitGroup
	var one, two *httptest.ResponseRecorder
	work.Go(func() { <-gate; one = f.request("PUT", "/api/connections/"+a.ID, first, 0) })
	work.Go(func() { <-gate; two = f.request("PUT", "/api/connections/"+b.ID, second, 0) })
	close(gate)
	work.Wait()
	if !(one.Code == 200 && two.Code == 400 || one.Code == 400 && two.Code == 200) {
		t.Fatalf("chain race: %d %d", one.Code, two.Code)
	}
}

func TestJumpSchemaAndRollback(t *testing.T) {
	f := setup(t)
	parent := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	fields := f.fields()
	fields.JumpConnectionID = &parent.ID
	child := result(t, f.request("POST", "/api/connections", fields, 0), 201)
	for _, statement := range []string{
		`UPDATE saved_connections SET jump_connection_id=id`,
		`UPDATE saved_connections SET jump_connection_id='missing'`,
		`UPDATE saved_connections SET jump_connection_id='` + child.ID + `' WHERE id='` + parent.ID + `'`,
		`DELETE FROM saved_connections WHERE id='` + parent.ID + `'`,
	} {
		if _, err := f.db.Exec(statement); err == nil {
			t.Fatal("invalid graph accepted", statement)
		}
	}
	if _, err := f.db.Exec(`INSERT OR REPLACE INTO saved_connections (id, account_id, name, host, port, username, ssh_key_id, created_at, updated_at) VALUES (?, ?, 'Foreign', 'host', 22, 'demo', ?, 1, 1)`, parent.ID, f.owners[1], f.keys[1]); err == nil {
		t.Fatal("replacement changed a referenced bastion's owner")
	}
	body, err := migrations.Files.ReadFile("20261004000200_jump_connections.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, down, _ := strings.Cut(string(body), "-- migrate:down")
	if _, err := f.db.Exec(down); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.db.QueryRow("SELECT count(*) FROM saved_connections").Scan(&count); err != nil || count != 2 {
		t.Fatalf("rollback lost connections: %d %v", count, err)
	}
	if _, err := f.db.Exec(up); err != nil {
		t.Fatal(err)
	}
	var jump sql.NullString
	if err := f.db.QueryRow("SELECT jump_connection_id FROM saved_connections WHERE id=?", child.ID).Scan(&jump); err != nil || jump.Valid {
		t.Fatalf("reapply not direct: %v %v", jump, err)
	}
	if _, err := f.db.Exec("UPDATE saved_connections SET jump_connection_id=? WHERE id=?", parent.ID, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("DELETE FROM accounts WHERE id=?", f.owners[0]); err != nil {
		t.Fatal("account cascade failed", err)
	}
	if err := f.db.QueryRow("SELECT count(*) FROM saved_connections").Scan(&count); err != nil || count != 0 {
		t.Fatal("account cascade retained connections")
	}
}

func TestJumpCannotFallBackToDirectDial(t *testing.T) {
	f := setup(t)
	d := NewDialer(f.db, nil)
	_, err := d.exchange(t.Context(), queries.SavedConnection{AccountID: f.owners[0], JumpConnectionID: sql.NullString{String: uuid.NewString(), Valid: true}}, nil)
	requireStatus(t, err, 404)
}
