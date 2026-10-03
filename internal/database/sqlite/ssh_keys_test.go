package sqlite_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/db/migrations"
	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
)

func TestSSHKeySchema(t *testing.T) {
	db, path := testdb.New(t)
	ctx := t.Context()
	const owner = "11111111-1111-4111-8111-111111111111"
	const otherOwner = "22222222-2222-4222-8222-222222222222"
	const keyID = "33333333-3333-4333-8333-333333333333"
	q := queries.New(db)
	for i, id := range []string{owner, otherOwner} {
		if err := q.CreateAccount(ctx, queries.CreateAccountParams{
			ID: id, DisplayName: "Key owner", RpID: "localhost",
			WebauthnUserHandle: []byte{byte(i)}, CreatedAt: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Arbitrary bytes exercise BLOB storage, not encryption, which is separate.
	want := queries.SshKey{
		ID: keyID, AccountID: owner, Name: "Local lab",
		PublicFingerprint:   "SHA256:" + strings.Repeat("A", 43),
		EncryptedPrivateKey: []byte{0, 255, 128, 1, 0},
		CreatedAt:           time.Date(2026, 10, 3, 12, 0, 0, 123456789, time.UTC).UnixNano(),
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO ssh_keys
		(id, account_id, name, public_fingerprint, encrypted_private_key, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, want.ID, want.AccountID, want.Name, want.PublicFingerprint, want.EncryptedPrivateKey, want.CreatedAt); err != nil {
		t.Fatal(err)
	}

	for _, assignment := range []string{
		"id = NULL", "id = 'not-a-uuid'", "id = 'AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA'",
		"account_id = NULL", "account_id = '99999999-9999-4999-8999-999999999999'",
		"name = NULL", "name = ''", "name = '   '",
		"public_fingerprint = NULL", "public_fingerprint = ''", "public_fingerprint = ' '",
		"encrypted_private_key = NULL", "encrypted_private_key = X''", "encrypted_private_key = 'text'",
		"created_at = NULL", "created_at = 'not an integer'", "created_at = 1.5",
	} {
		t.Run(assignment, func(t *testing.T) {
			if _, err := db.ExecContext(ctx, "UPDATE ssh_keys SET "+assignment); err == nil {
				t.Fatal("invalid key record accepted")
			}
		})
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO ssh_keys SELECT * FROM ssh_keys"); err == nil {
		t.Fatal("duplicate key ID accepted")
	}
	// Names and fingerprints are labels, not unique identities, within or across accounts.
	for i, accountID := range []string{owner, otherOwner} {
		id := []string{"44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555"}[i]
		if _, err := db.ExecContext(ctx, `INSERT INTO ssh_keys
			SELECT ?, ?, name, public_fingerprint, encrypted_private_key, created_at FROM ssh_keys WHERE id = ?`, id, accountID, keyID); err != nil {
			t.Fatal("duplicate metadata rejected:", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var got queries.SshKey
	if err := db.QueryRowContext(ctx, `SELECT id, account_id, name, public_fingerprint, encrypted_private_key, created_at
		FROM ssh_keys WHERE id = ?`, keyID).Scan(&got.ID, &got.AccountID, &got.Name, &got.PublicFingerprint, &got.EncryptedPrivateKey, &got.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("SSH key metadata or encrypted bytes changed after reopen")
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", owner); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM ssh_keys").Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("cascade left %d keys, want only the other account's key: %v", remaining, err)
	}
	var remainingOwner string
	if err := db.QueryRowContext(ctx, "SELECT account_id FROM ssh_keys").Scan(&remainingOwner); err != nil || remainingOwner != otherOwner {
		t.Fatalf("remaining owner = %q: %v", remainingOwner, err)
	}
}

func TestSSHKeyMigrationRollbackPreservesAccounts(t *testing.T) {
	db, _ := testdb.New(t)
	ctx := t.Context()
	const owner = "11111111-1111-4111-8111-111111111111"
	account := queries.CreateAccountParams{ID: owner, DisplayName: "Existing account", RpID: "localhost", WebauthnUserHandle: []byte{0, 255}, CreatedAt: 1}
	if err := queries.New(db).CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO sessions VALUES ('existing-session', X'00ff', 123)"); err != nil {
		t.Fatal(err)
	}
	body, err := migrations.Files.ReadFile("20261003000400_ssh_keys.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, down, ok := strings.Cut(string(body), "-- migrate:down")
	if !ok {
		t.Fatal("missing rollback")
	}
	if _, err := db.ExecContext(ctx, down); err != nil {
		t.Fatal(err)
	}
	var tables int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name = 'ssh_keys'").Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("key table remains after rollback: %v", err)
	}
	if _, err := db.ExecContext(ctx, up); err != nil {
		t.Fatal(err)
	}
	identity, err := queries.New(db).GetSessionAccount(ctx, queries.GetSessionAccountParams{ID: owner, RpID: "localhost"})
	if err != nil || identity.DisplayName != account.DisplayName {
		t.Fatalf("account did not survive rollback/reapply: %v", err)
	}
	var data []byte
	if err := db.QueryRowContext(ctx, "SELECT data FROM sessions WHERE token = 'existing-session'").Scan(&data); err != nil || !bytes.Equal(data, []byte{0, 255}) {
		t.Fatalf("session did not survive rollback/reapply: %v", err)
	}
}
