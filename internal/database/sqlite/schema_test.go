package sqlite_test

import (
	"bytes"
	"database/sql"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/db/migrations"
	"github.com/carlyleec/go-ssh-term/internal/auth/sqlitestore"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
)

func TestMigrationHistory(t *testing.T) {
	db := migrated(t)
	check := func(want string) {
		t.Helper()
		err := sqlite.CheckMigrations(t.Context(), db)
		if want == "" {
			if err != nil {
				t.Fatal(err)
			}
			return
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("want %q, got %v", want, err)
		}
	}
	check("not initialized")
	if _, err := db.Exec("CREATE TABLE schema_migrations (version TEXT PRIMARY KEY NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	check("pending")
	for _, file := range activeMigrations(t) {
		check("pending")
		version, _, _ := strings.Cut(file, "_")
		if _, err := db.Exec("INSERT INTO schema_migrations VALUES (?)", version); err != nil {
			t.Fatal(err)
		}
	}
	check("")
	for _, version := range []string{"20261003000100", "20261003000200", "20990101000000"} {
		if _, err := db.Exec("INSERT INTO schema_migrations VALUES (?)", version); err != nil {
			t.Fatal(err)
		}
		check("unknown")
		if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = ?", version); err != nil {
			t.Fatal(err)
		}
	}
}

func activeMigrations(t *testing.T) []string {
	t.Helper()
	files, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 || files[0] != "20261003000300_account_access.sql" {
		t.Fatalf("unexpected active lineage: %v", files)
	}
	return files
}

func migrated(t *testing.T) *sql.DB {
	t.Helper()
	db := rawOpen(t, filepath.Join(t.TempDir(), "schema.db"))
	for _, file := range activeMigrations(t) {
		body, err := migrations.Files.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(body), "-- migrate:down")
		if _, err := db.ExecContext(t.Context(), up); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestSchemaAndGeneratedQueries(t *testing.T) {
	db := migrated(t)
	q := queries.New(db)
	ctx := t.Context()
	const first = "11111111-1111-4111-8111-111111111111"
	const second = "22222222-2222-4222-8222-222222222222"
	const third = "33333333-3333-4333-8333-333333333333"
	timestamp := time.Date(2026, 10, 3, 12, 34, 56, 123456789, time.UTC).UnixNano()
	account := queries.CreateAccountParams{ID: first, DisplayName: "Same name", RpID: "localhost", WebauthnUserHandle: []byte{0, 255, 1}, CreatedAt: timestamp}
	if err := q.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	other := account
	other.ID = second
	other.WebauthnUserHandle = []byte{2}
	if err := q.CreateAccount(ctx, other); err != nil {
		t.Fatal(err)
	}
	other.ID = third
	other.RpID = "other.example"
	other.WebauthnUserHandle = account.WebauthnUserHandle
	if err := q.CreateAccount(ctx, other); err != nil {
		t.Fatal("handles must be RP scoped:", err)
	}
	credential := queries.CreatePasskeyCredentialParams{
		AccountID: first, RpID: "localhost", CredentialID: []byte{0, 255, 2}, PublicKey: []byte{0, 255, 3},
		AttestationType: "basic", AttestationFormat: "packed", Transports: `["internal","usb"]`, Flags: 255,
		Aaguid: bytes.Repeat([]byte{255}, 16), SignCount: 4294967295, CloneWarning: 1, Attachment: "platform",
		Attestation: `{"clientDataJSON":"AQID","object":"BAUG"}`, Extensions: `{"credProps":{"rk":true}}`, CreatedAt: timestamp,
	}
	if err := q.CreatePasskeyCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	lookup := queries.GetLoginCredentialParams{RpID: "localhost", CredentialID: credential.CredentialID, WebauthnUserHandle: account.WebauthnUserHandle}
	got, err := q.GetLoginCredential(ctx, lookup)
	if err != nil {
		t.Fatal(err)
	}
	wantAccount := queries.Account{ID: first, DisplayName: account.DisplayName, RpID: account.RpID, WebauthnUserHandle: account.WebauthnUserHandle, CreatedAt: timestamp}
	wantCredential := queries.PasskeyCredential{
		AccountID: first, RpID: credential.RpID, CredentialID: credential.CredentialID, PublicKey: credential.PublicKey,
		AttestationType: credential.AttestationType, AttestationFormat: credential.AttestationFormat, Transports: credential.Transports,
		Flags: credential.Flags, Aaguid: credential.Aaguid, SignCount: credential.SignCount, CloneWarning: credential.CloneWarning,
		Attachment: credential.Attachment, Attestation: credential.Attestation, Extensions: credential.Extensions, CreatedAt: timestamp,
	}
	if !reflect.DeepEqual(got.Account, wantAccount) || !reflect.DeepEqual(got.PasskeyCredential, wantCredential) {
		t.Fatalf("metadata round trip: %#v", got)
	}
	last := sql.NullInt64{Int64: timestamp + 1, Valid: true}
	if err := q.UpdateLoginCredential(ctx, queries.UpdateLoginCredentialParams{AccountID: first, RpID: "localhost", SignCount: 42, CloneWarning: 0, Flags: 29, LastUsedAt: last}); err != nil {
		t.Fatal(err)
	}
	got, err = q.GetLoginCredential(ctx, lookup)
	if err != nil || got.PasskeyCredential.LastUsedAt != last || got.PasskeyCredential.SignCount != 42 || got.PasskeyCredential.Flags != 29 || got.PasskeyCredential.CloneWarning != 0 {
		t.Fatalf("login update: %#v, %v", got, err)
	}
	identity, err := q.GetSessionAccount(ctx, queries.GetSessionAccountParams{ID: first, RpID: "localhost"})
	if err != nil || identity.ID != first || identity.DisplayName != account.DisplayName {
		t.Fatalf("identity: %#v, %v", identity, err)
	}
	if _, err := q.GetSessionAccount(ctx, queries.GetSessionAccountParams{ID: first, RpID: "other.example"}); err != sql.ErrNoRows {
		t.Fatalf("RP isolation: %v", err)
	}

	t.Run("constraints", func(t *testing.T) {
		for _, statement := range []string{
			`UPDATE accounts SET id = 'not-a-uuid' WHERE id = '` + second + `'`,
			`UPDATE accounts SET id = 'AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA' WHERE id = '` + second + `'`,
			`UPDATE accounts SET display_name = ' '`, `UPDATE accounts SET rp_id = ''`,
			`UPDATE accounts SET webauthn_user_handle = X''`, `UPDATE accounts SET webauthn_user_handle = zeroblob(65)`,
			`UPDATE accounts SET webauthn_user_handle = 'text'`, `UPDATE accounts SET created_at = 'not an integer'`,
			`UPDATE accounts SET webauthn_user_handle = X'00ff01' WHERE id = '` + second + `'`,
			`UPDATE passkey_credentials SET account_id = '` + third + `'`,
			`UPDATE passkey_credentials SET credential_id = X''`, `UPDATE passkey_credentials SET public_key = X''`,
			`UPDATE passkey_credentials SET aaguid = zeroblob(15)`,
			`UPDATE passkey_credentials SET sign_count = -1`, `UPDATE passkey_credentials SET sign_count = 4294967296`,
			`UPDATE passkey_credentials SET flags = -1`, `UPDATE passkey_credentials SET flags = 256`,
			`UPDATE passkey_credentials SET clone_warning = 2`,
			`UPDATE passkey_credentials SET transports = '{}'`, `UPDATE passkey_credentials SET transports = 'invalid'`,
			`UPDATE passkey_credentials SET attestation = '[]'`, `UPDATE passkey_credentials SET extensions = 'null'`,
			`UPDATE passkey_credentials SET attestation = 'invalid'`, `UPDATE passkey_credentials SET extensions = 'invalid'`,
			`INSERT INTO sessions VALUES (NULL, X'01', 1)`, `INSERT INTO sessions VALUES ('bad', 'text', 1)`,
		} {
			if _, err := db.ExecContext(ctx, statement); err == nil {
				t.Errorf("accepted %s", statement)
			}
		}
		duplicate := credential
		duplicate.CredentialID = []byte{4}
		if err := q.CreatePasskeyCredential(ctx, duplicate); err == nil {
			t.Error("multiple credentials per account accepted")
		}
		duplicate = credential
		duplicate.AccountID = second
		if err := q.CreatePasskeyCredential(ctx, duplicate); err == nil {
			t.Error("duplicate RP credential accepted")
		}
		duplicate.AccountID = third
		duplicate.RpID = "other.example"
		if err := q.CreatePasskeyCredential(ctx, duplicate); err != nil {
			t.Fatal("credential IDs must be RP scoped:", err)
		}
	})
	t.Run("session adapter", func(t *testing.T) {
		store := sqlitestore.New(db, 0)
		defer store.StopCleanup()
		expiry := time.Now().Add(time.Hour).Truncate(time.Second).Add(123456789 * time.Nanosecond)
		data := []byte{0, 255, 1}
		if err := store.CommitCtx(ctx, "session", data, expiry); err != nil {
			t.Fatal(err)
		}
		var stored int64
		if err := db.QueryRowContext(ctx, "SELECT expiry FROM sessions WHERE token = ?", "session").Scan(&stored); err != nil || stored != expiry.UnixNano() {
			t.Fatalf("expiry precision: %d, %v", stored, err)
		}
		got, found, err := store.FindCtx(ctx, "session")
		if err != nil || !found || !bytes.Equal(got, data) {
			t.Fatalf("session round trip: %v, %v", found, err)
		}
		if err := store.CommitCtx(ctx, "session", []byte{2}, time.Now().Add(-time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, found, err := store.FindCtx(ctx, "session"); err != nil || found {
			t.Fatalf("expired session: %v, %v", found, err)
		}
	})
	// Force a replacement connection before exercising the composite FK cascade.
	db.SetMaxIdleConns(0)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", first); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetLoginCredential(ctx, lookup); err != sql.ErrNoRows {
		t.Fatalf("cascade after reconnect: %v", err)
	}
}

func TestSchemaRollbackAndReapply(t *testing.T) {
	db := migrated(t)
	files := activeMigrations(t)
	for _, file := range slices.Backward(files) {
		body, err := migrations.Files.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		_, down, ok := strings.Cut(string(body), "-- migrate:down")
		if !ok {
			t.Fatalf("missing rollback in %s", file)
		}
		if _, err := db.ExecContext(t.Context(), down); err != nil {
			t.Fatal(err)
		}
	}
	var tables int
	if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM sqlite_schema WHERE type = 'table'").Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("rollback left %d tables: %v", tables, err)
	}
	for _, file := range files {
		body, err := migrations.Files.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(body), "-- migrate:down")
		if _, err := db.ExecContext(t.Context(), up); err != nil {
			t.Fatal(err)
		}
	}
}
