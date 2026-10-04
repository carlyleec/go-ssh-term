package sqlite_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/carlyleec/go-ssh-term/db/migrations"
	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
)

func TestConnectionSchema(t *testing.T) {
	db, path := testdb.New(t)
	ctx := t.Context()
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	id := func(n int) string { return fmt.Sprintf("%08d-1111-4111-8111-111111111111", n) }
	const timestamp int64 = 1791028800123456789
	for n := 1; n <= 2; n++ {
		exec(`INSERT INTO accounts VALUES (?, 'Owner', 'localhost', ?, ?)`, id(n), []byte{byte(n)}, timestamp)
		exec(`INSERT INTO ssh_keys VALUES (?, ?, 'Key', 'SHA256:demo', X'01ff', ?)`, id(n+2), id(n), timestamp)
		exec(`INSERT INTO saved_connections VALUES (?, ?, 'Lab', 'bastion', 22, 'demo', ?, ?, ?)`, id(n+4), id(n), id(n+2), timestamp, timestamp)
		exec(`INSERT INTO host_trust VALUES (?, 'bastion', 22, ?, ?)`, id(n), []byte{0, 255, 128}, timestamp)
		exec(`INSERT INTO connection_audit_events VALUES (?, ?, ?, ?, 'Lab', 'bastion', 22, 'demo', 'start', NULL, ?)`, id(n+6), id(n), id(n+4), id(n+8), timestamp)
	}
	for _, tc := range []struct{ table, assignment string }{
		{"saved_connections", "id = 'invalid'"},
		{"saved_connections", "ssh_key_id = '" + id(4) + "'"},
		{"saved_connections", "ssh_key_id = 'missing'"},
		{"saved_connections", "account_id = 'missing'"},
		{"saved_connections", "name = ' '"},
		{"saved_connections", "host = ''"},
		{"saved_connections", "username = ''"},
		{"saved_connections", "port = 0"},
		{"saved_connections", "port = 65536"},
		{"saved_connections", "port = 1.5"},
		{"saved_connections", "updated_at = 0"},
		{"saved_connections", "created_at = NULL"},
		{"host_trust", "public_key = X''"},
		{"host_trust", "public_key = 'text'"},
		{"host_trust", "host = ' '"},
		{"host_trust", "port = 0"},
		{"host_trust", "trusted_at = 'bad'"},
		{"connection_audit_events", "id = 'invalid'"},
		{"connection_audit_events", "saved_connection_id = 'invalid'"},
		{"connection_audit_events", "attempt_id = 'invalid'"},
		{"connection_audit_events", "event_type = 'unknown'"},
		{"connection_audit_events", "event_type = 'failure'"},
		{"connection_audit_events", "failure_code = 'unexpected'"},
		{"connection_audit_events", "event_type = 'failure', failure_code = 'raw error with secrets'"},
		{"connection_audit_events", "event_type = 'failure', failure_code = ''"},
		{"connection_audit_events", "host = ''"},
		{"connection_audit_events", "port = 65536"},
		{"connection_audit_events", "occurred_at = NULL"},
	} {
		t.Run(tc.table+"/"+tc.assignment, func(t *testing.T) {
			if _, err := db.ExecContext(ctx, "UPDATE "+tc.table+" SET "+tc.assignment+" WHERE account_id = ?", id(1)); err == nil {
				t.Fatal("invalid record accepted")
			}
		})
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id = ?`, id(3)); err == nil {
		t.Fatal("referenced key deleted")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO host_trust SELECT * FROM host_trust WHERE account_id = ?`, id(1)); err == nil {
		t.Fatal("duplicate endpoint trust accepted")
	}
	// Endpoint trust is independent of connection names, usernames, and port aliases.
	exec(`INSERT INTO host_trust SELECT account_id, host, 2222, public_key, trusted_at FROM host_trust WHERE account_id = ?`, id(1))
	exec(`UPDATE saved_connections SET name = 'Renamed', host = 'elsewhere', port = 2222 WHERE id = ?`, id(5))
	exec(`DELETE FROM saved_connections WHERE id = ?`, id(5))
	exec(`DELETE FROM ssh_keys WHERE id = ?`, id(3))
	// A shell's final event can arrive after its saved connection has been removed.
	for n, event := range []string{"end", "failure"} {
		var code any
		if event == "failure" {
			code = "dial_timeout"
		}
		exec(`INSERT INTO connection_audit_events VALUES (?, ?, ?, ?, 'Lab', 'bastion', 22, 'demo', ?, ?, ?)`, id(11+n), id(1), id(5), id(9), event, code, timestamp+1)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	db, err = database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name, host, username string
	var port, at int64
	if err := db.QueryRowContext(ctx, `SELECT connection_name, host, port, username, occurred_at FROM connection_audit_events WHERE id = ?`, id(7)).Scan(&name, &host, &port, &username, &at); err != nil {
		t.Fatal(err)
	}
	if name != "Lab" || host != "bastion" || port != 22 || username != "demo" || at != timestamp {
		t.Fatal("audit snapshot changed after edit/delete/reopen")
	}
	var key []byte
	if err := db.QueryRowContext(ctx, `SELECT public_key, trusted_at FROM host_trust WHERE account_id = ? AND port = 22`, id(1)).Scan(&key, &at); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, []byte{0, 255, 128}) || at != timestamp {
		t.Fatal("trust bytes or timestamp changed")
	}
	// Account deletion must cascade through both keys and referencing connections.
	exec(`DELETE FROM accounts WHERE id = ?`, id(2))
	for _, table := range []string{"ssh_keys", "saved_connections", "host_trust", "connection_audit_events"} {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE account_id = ?", id(2)).Scan(&count); err != nil || count != 0 {
			t.Fatalf("cascade %s: %d, %v", table, count, err)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM connection_audit_events WHERE account_id = ?`, id(1)).Scan(&count); err != nil || count != 3 {
		t.Fatalf("other account audit changed: %d, %v", count, err)
	}
}

func TestConnectionMigrationRollbackPreservesExistingData(t *testing.T) {
	db, _ := testdb.New(t)
	ctx := t.Context()
	for _, statement := range []string{
		`INSERT INTO accounts VALUES ('11111111-1111-4111-8111-111111111111', 'Owner', 'localhost', X'01', 1)`,
		`INSERT INTO ssh_keys VALUES ('22222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111', 'Key', 'SHA256:demo', X'01ff', 1)`,
		`INSERT INTO sessions VALUES ('existing-session', X'00ff', 123)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	body, err := migrations.Files.ReadFile("20261004000100_connections.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, down, ok := strings.Cut(string(body), "-- migrate:down")
	if !ok {
		t.Fatal("missing rollback")
	}
	for _, statement := range []string{down, up} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"accounts", "ssh_keys", "sessions"} {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s lost on rollback/reapply: %d, %v", table, count, err)
		}
	}
}
