package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/carlyleec/go-ssh-term/db/migrations"
	"github.com/carlyleec/go-ssh-term/internal/database/queries"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run registration persistence checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	name := "registration_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err = pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	files, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		body, err := migrations.Files.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(body), "-- migrate:down")
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func persistedRegistrationFixture(t *testing.T, pool *pgxpool.Pool) (*registration, http.Handler) {
	t.Helper()
	h, handler := registrationFixture(t)
	h.sessions.Store = pgxstore.NewWithCleanupInterval(pool, 0)
	h.save = func(ctx context.Context, u registrationUser, c *webauthn.Credential) error {
		return saveRegistration(ctx, pool, "localhost", u, c)
	}
	return h, handler
}

func registrationCounts(t *testing.T, pool *pgxpool.Pool, want int) {
	t.Helper()
	var accounts, credentials int
	if err := pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM accounts), (SELECT count(*) FROM passkey_credentials)").Scan(&accounts, &credentials); err != nil {
		t.Fatal(err)
	}
	if accounts != want || credentials != want {
		t.Fatalf("accounts=%d credentials=%d want=%d", accounts, credentials, want)
	}
}

func storedCredential(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *webauthn.Credential {
	t.Helper()
	var row queries.PasskeyCredential
	err := pool.QueryRow(context.Background(), `SELECT credential_id,public_key,attestation_type,attestation_format,transports,flags,aaguid,sign_count,clone_warning,attachment,attestation,extensions FROM passkey_credentials WHERE account_id=$1`, id.String()).Scan(&row.CredentialID, &row.PublicKey, &row.AttestationType, &row.AttestationFormat, &row.Transports, &row.Flags, &row.Aaguid, &row.SignCount, &row.CloneWarning, &row.Attachment, &row.Attestation, &row.Extensions)
	if err != nil {
		t.Fatal(err)
	}
	c := &webauthn.Credential{
		ID: row.CredentialID, PublicKey: row.PublicKey, AttestationType: row.AttestationType, AttestationFormat: row.AttestationFormat,
		Flags:         webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(row.Flags)),
		Authenticator: webauthn.Authenticator{AAGUID: row.Aaguid, SignCount: uint32(row.SignCount), CloneWarning: row.CloneWarning, Attachment: protocol.AuthenticatorAttachment(row.Attachment)},
	}
	for _, transport := range row.Transports {
		c.Transport = append(c.Transport, protocol.AuthenticatorTransport(transport))
	}
	if err := json.Unmarshal(row.Attestation, &c.Attestation); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(row.Extensions, &c.Extensions); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRegistrationPostgres(t *testing.T) {
	pool := registrationDatabase(t)
	t.Run("verified account and session", func(t *testing.T) {
		h, handler := persistedRegistrationFixture(t, pool)
		options, cookie := beginTest(t, handler, nil)
		registrationCounts(t, pool, 0)
		var pending pendingRegistration
		for _, p := range h.pending {
			pending = p
		}
		body := credentialResponse(t, options, testOrigin, "localhost", 0x45)
		expected, err := h.webauthn.FinishRegistration(pending.user, pending.session, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if err != nil {
			t.Fatal(err)
		}
		w := registrationRequestTest(handler, "finish", body, testOrigin, cookie)
		if w.Code != 201 {
			t.Fatalf("finish: %d %s", w.Code, w.Body.String())
		}
		registrationCounts(t, pool, 1)
		if !reflect.DeepEqual(storedCredential(t, pool, pending.user.ID), expected) {
			t.Fatal("verified credential metadata was not preserved")
		}
		var name, rp string
		var handle []byte
		if err := pool.QueryRow(context.Background(), "SELECT display_name,rp_id,webauthn_user_handle FROM accounts WHERE id=$1", pending.user.ID.String()).Scan(&name, &rp, &handle); err != nil {
			t.Fatal(err)
		}
		if name != pending.user.DisplayName || rp != "localhost" || !bytes.Equal(handle, pending.user.Handle) {
			t.Fatal("pending account identity was not preserved")
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Value == cookie.Value || sessionAccount(t, h, cookies[0]) != pending.user.ID.String() || sessionAccount(t, h, cookie) != "" {
			t.Fatal("Postgres session did not rotate and authenticate")
		}
		if retry := registrationRequestTest(handler, "finish", body, testOrigin, cookie); retry.Code != 400 {
			t.Fatal("registration replay accepted")
		}
		// The same credential ID under a new verified challenge cannot create an orphan account.
		options, cookie = beginTest(t, handler, nil)
		body = credentialResponse(t, options, testOrigin, "localhost", 0x45)
		w = registrationRequestTest(handler, "finish", body, testOrigin, cookie)
		if w.Code != 409 || len(w.Result().Cookies()) != 0 || sessionAccount(t, h, cookie) != "" {
			t.Fatalf("duplicate credential: %d %s", w.Code, w.Body.String())
		}
		registrationCounts(t, pool, 1)
		// Preserve non-default assertion metadata and optional extension values too.
		metadata := *expected
		metadata.ID = []byte("second-credential")
		metadata.Transport = nil
		metadata.Flags = webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(0x1d))
		metadata.Authenticator.SignCount = 4294967295
		metadata.Authenticator.CloneWarning = true
		resident := true
		minimumPINLength := uint(6)
		metadata.Extensions.RK = &resident
		metadata.Extensions.MinPinLength = &minimumPINLength
		second := registrationUser{ID: uuid.New(), Handle: []byte("second-user-handle"), DisplayName: pending.user.DisplayName}
		if err := saveRegistration(context.Background(), pool, "localhost", second, &metadata); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(storedCredential(t, pool, second.ID), &metadata) {
			t.Fatal("non-default credential metadata was not preserved")
		}
		registrationCounts(t, pool, 2)
	})
	for _, scenario := range []string{"invalid verification", "commit failure", "session commit failure", "concurrent finish"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			if _, err := pool.Exec(ctx, "TRUNCATE accounts, passkey_credentials, sessions"); err != nil {
				t.Fatal(err)
			}
			h, _ := persistedRegistrationFixture(t, pool)
			handler := NewRegistration(h.webauthn, h.sessions, pool, testOrigin)
			options, cookie := beginTest(t, handler, nil)
			body := credentialResponse(t, options, testOrigin, "localhost", 0x45)
			switch scenario {
			case "invalid verification":
				body = credentialResponse(t, options, testOrigin, "localhost", 0x41)
				if w := registrationRequestTest(handler, "finish", body, testOrigin, cookie); w.Code != 400 {
					t.Fatal("invalid verification accepted")
				}
				registrationCounts(t, pool, 0)
			case "commit failure":
				_, err := pool.Exec(ctx, `CREATE FUNCTION reject_registration_commit() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'test commit failure'; END$$;
CREATE CONSTRAINT TRIGGER reject_commit AFTER INSERT ON passkey_credentials DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_registration_commit();`)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := pool.Exec(ctx, "DROP TRIGGER reject_commit ON passkey_credentials; DROP FUNCTION reject_registration_commit()"); err != nil {
						t.Error(err)
					}
				})
				w := registrationRequestTest(handler, "finish", body, testOrigin, cookie)
				if w.Code != 503 || len(w.Result().Cookies()) != 0 || sessionAccount(t, h, cookie) != "" {
					t.Fatal("failed transaction granted access")
				}
				registrationCounts(t, pool, 0)
				if retry := registrationRequestTest(handler, "finish", body, testOrigin, cookie); retry.Code != 400 {
					t.Fatal("failed transaction allowed replay")
				}
			case "session commit failure":
				store := h.sessions.Store
				h.sessions.Store = &failingSessionStore{Store: store, failCommit: true}
				w := registrationRequestTest(handler, "finish", body, testOrigin, cookie)
				h.sessions.Store = store
				if w.Code != 503 || !strings.Contains(w.Body.String(), "account saved") || strings.Contains(w.Body.String(), `"account":`) || len(w.Result().Cookies()) != 0 || sessionAccount(t, h, cookie) != "" {
					t.Fatalf("session failure: %d %s", w.Code, w.Body.String())
				}
				registrationCounts(t, pool, 1)
			case "concurrent finish":
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
				for code := range results {
					if code == 201 {
						successes++
					} else if code != 400 {
						t.Fatalf("unexpected concurrent status %d", code)
					}
				}
				if successes != 1 {
					t.Fatalf("expected one creation, got %d", successes)
				}
				registrationCounts(t, pool, 1)
			}
		})
	}
}
