package database

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/carlyleec/go-ssh-term/db/migrations"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAccountSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	const first = "11111111-1111-4111-8111-111111111111"
	const second = "22222222-2222-4222-8222-222222222222"
	const missing = "33333333-3333-4333-8333-333333333333"
	const accountSQL = `INSERT INTO accounts (id, display_name, rp_id, webauthn_user_handle) VALUES ($1, $2, $3, $4)`
	for i, id := range []string{first, second} {
		if _, err := pool.Exec(ctx, accountSQL, id, "Same display name", "localhost", []byte{byte(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	expectError := func(t *testing.T, query, code string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != code {
			t.Fatalf("expected PostgreSQL %s, got %v", code, err)
		}
	}
	t.Run("account constraints", func(t *testing.T) {
		expectError(t, accountSQL, "23505", missing, "Different name", "localhost", []byte{1})
		expectError(t, accountSQL, "23514", missing, " ", "localhost", []byte{3})
		expectError(t, accountSQL, "23514", missing, "Name", "localhost", []byte{})
		expectError(t, accountSQL, "23514", missing, "Name", "localhost", make([]byte, 65))
	})
	const credentialSQL = `INSERT INTO passkey_credentials
 (account_id, rp_id, credential_id, public_key, attestation_type, attestation_format,
 flags, aaguid, attestation, extensions)
 VALUES ($1, $2, $3, $4, 'none', 'none', 29, $5,
 '{"clientDataJSON":"AQID","object":"BAUG"}', '{"rk":true}')`
	insertCredential := func(account, rp string, id []byte) error {
		_, err := pool.Exec(ctx, credentialSQL, account, rp, id, []byte{0, 255, 1}, make([]byte, 16))
		return err
	}
	if err := insertCredential(first, "localhost", []byte{0, 255}); err != nil {
		t.Fatal(err)
	}
	t.Run("credential ownership and metadata", func(t *testing.T) {
		for _, tc := range []struct {
			account, rp, code string
			id                []byte
		}{
			{first, "localhost", "23505", []byte{2}},
			{second, "localhost", "23505", []byte{0, 255}},
			{missing, "localhost", "23503", []byte{3}},
			{second, "other.example", "23503", []byte{4}},
		} {
			err := insertCredential(tc.account, tc.rp, tc.id)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
		}
		expectError(t, "UPDATE passkey_credentials SET sign_count = -1", "23514")
		expectError(t, "UPDATE passkey_credentials SET sign_count = 4294967296", "23514")
		expectError(t, "UPDATE passkey_credentials SET flags = 256", "23514")
		if _, err := pool.Exec(ctx, "UPDATE passkey_credentials SET sign_count = 4294967295, flags = 255, last_used_at = CURRENT_TIMESTAMP"); err != nil {
			t.Fatal(err)
		}
		var key []byte
		var count int64
		var flags int16
		var clientData string
		if err := pool.QueryRow(ctx, "SELECT public_key, sign_count, flags, attestation->>'clientDataJSON' FROM passkey_credentials WHERE account_id = $1", first).Scan(&key, &count, &flags, &clientData); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(key, []byte{0, 255, 1}) || count != 4294967295 || flags != 255 || clientData != "AQID" {
			t.Fatal("credential metadata did not round-trip")
		}
	})
	t.Run("pgxstore compatibility", func(t *testing.T) {
		store := pgxstore.NewWithCleanupInterval(pool, 0)
		data := []byte{0, 1, 255}
		if err := store.CommitCtx(ctx, "live", data, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		got, found, err := store.FindCtx(ctx, "live")
		if err != nil || !found || !bytes.Equal(got, data) {
			t.Fatalf("session round-trip: found=%v err=%v", found, err)
		}
		if err := store.CommitCtx(ctx, "live", []byte{2}, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		got, found, err = store.FindCtx(ctx, "live")
		if err != nil || !found || !bytes.Equal(got, []byte{2}) {
			t.Fatal("session update failed")
		}
		if err := store.CommitCtx(ctx, "expired", data, time.Now().Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, found, err := store.FindCtx(ctx, "expired"); err != nil || found {
			t.Fatal("expired session was returned")
		}
		if err := store.DeleteCtx(ctx, "live"); err != nil {
			t.Fatal(err)
		}
		if _, found, err := store.FindCtx(ctx, "live"); err != nil || found {
			t.Fatal("deleted session was returned")
		}
	})
	t.Run("account deletion", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "DELETE FROM accounts WHERE id = $1", first); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM passkey_credentials").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("credential was not removed with its account")
		}
	})
	t.Run("rollback and reapply", func(t *testing.T) {
		body, err := migrations.Files.ReadFile("20261003000200_account_access.sql")
		if err != nil {
			t.Fatal(err)
		}
		parts := bytes.SplitN(body, []byte("-- migrate:down"), 2)
		if _, err := pool.Exec(ctx, string(parts[1])); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"accounts", "passkey_credentials", "sessions"} {
			var absent bool
			if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NULL", table).Scan(&absent); err != nil || !absent {
				t.Fatalf("table %s survived rollback: %v", table, err)
			}
		}
		if _, err := pool.Exec(ctx, string(parts[0])); err != nil {
			t.Fatal(err)
		}
	})
}
