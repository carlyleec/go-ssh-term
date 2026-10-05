package auth

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/carlyleec/go-ssh-term/internal/auth/sqlitestore"
	"github.com/google/uuid"
)

func loginDatabaseFixture(t *testing.T, pool *sql.DB) (*Login, http.Handler, loginUser, *ecdsa.PrivateKey) {
	t.Helper()
	h, _, user, key := loginFixture(t)
	h.sessions.Store = sqlitestore.New(pool, 0)
	if err := saveRegistration(context.Background(), pool, "localhost", registrationUser{ID: uuid.MustParse(user.account.ID), Handle: user.account.WebauthnUserHandle, DisplayName: user.account.DisplayName}, &user.credential); err != nil {
		t.Fatal(err)
	}
	return h, NewLogin(h.webauthn, h.sessions, pool).routes(testOrigin), user, key
}

func TestLoginSQLite(t *testing.T) {
	pool := registrationDatabase(t)
	for _, scenario := range []string{"metadata and sessions", "identity scope", "failed verification", "update failure", "commit failure", "session failure", "concurrent metadata"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			if _, err := pool.ExecContext(ctx, "DELETE FROM accounts; DELETE FROM sessions"); err != nil {
				t.Fatal(err)
			}
			h, handler, user, key := loginDatabaseFixture(t, pool)
			options, cookie := beginLoginTest(t, handler, nil)
			body := assertionResponse(t, key, user, options, 1, 0x1d, testOrigin, "localhost")
			switch scenario {
			case "metadata and sessions":
				w := loginRequestTest(handler, "finish", body, testOrigin, cookie)
				if w.Code != 200 {
					t.Fatalf("finish: %d %s", w.Code, w.Body.String())
				}
				cookies := w.Result().Cookies()
				if len(cookies) != 1 || cookies[0].Value == cookie.Value {
					t.Fatal("session was not replaced")
				}
				session, err := h.sessions.Load(ctx, cookies[0].Value)
				if err != nil {
					t.Fatal(err)
				}
				if h.sessions.GetString(session, accountIDKey) != user.account.ID || h.sessions.Exists(session, loginBinding) {
					t.Fatal("incorrect login session")
				}
				var count int64
				var flags int16
				var used bool
				if err := pool.QueryRowContext(ctx, "SELECT sign_count,flags,last_used_at IS NOT NULL FROM passkey_credentials").Scan(&count, &flags, &used); err != nil {
					t.Fatal(err)
				}
				if count != 1 || flags&0x1d != 0x1d || !used {
					t.Fatal("login metadata was not saved")
				}
				// Zero counters are supported, and a non-increasing nonzero counter is advisory.
				options, cookie = beginLoginTest(t, handler, nil)
				w = loginRequestTest(handler, "finish", assertionResponse(t, key, user, options, 1, 0x0d, testOrigin, "localhost"), testOrigin, cookie)
				if w.Code != 200 {
					t.Fatal("counter warning unexpectedly rejected a valid signature")
				}
				var warning bool
				if err := pool.QueryRowContext(ctx, "SELECT sign_count,clone_warning,flags FROM passkey_credentials").Scan(&count, &warning, &flags); err != nil {
					t.Fatal(err)
				}
				if count != 1 || !warning || flags&0x10 != 0 {
					t.Fatal("counter warning or backup state was not persisted")
				}
				session, err = h.sessions.Load(ctx, cookies[0].Value)
				if err != nil || h.sessions.GetString(session, accountIDKey) == "" {
					t.Fatal("another login invalidated the first login session")
				}
				if _, err := pool.ExecContext(ctx, "UPDATE passkey_credentials SET sign_count=0,clone_warning=false"); err != nil {
					t.Fatal(err)
				}
				options, cookie = beginLoginTest(t, handler, nil)
				if w := loginRequestTest(handler, "finish", assertionResponse(t, key, user, options, 0, 0x0d, testOrigin, "localhost"), testOrigin, cookie); w.Code != 200 {
					t.Fatal("zero counter rejected")
				}
			case "identity scope":
				second, secondKey := loginIdentity(t)
				if err := saveRegistration(ctx, pool, "localhost", registrationUser{ID: uuid.MustParse(second.account.ID), Handle: second.account.WebauthnUserHandle, DisplayName: second.account.DisplayName}, &second.credential); err != nil {
					t.Fatal(err)
				}
				wrong := user
				wrong.account.WebauthnUserHandle = second.account.WebauthnUserHandle
				if w := loginRequestTest(handler, "finish", assertionResponse(t, key, wrong, options, 1, 0x1d, testOrigin, "localhost"), testOrigin, cookie); w.Code != 401 {
					t.Fatal("another account's handle accepted")
				}
				options, cookie = beginLoginTest(t, handler, nil)
				w := loginRequestTest(handler, "finish", assertionResponse(t, secondKey, second, options, 1, 0x1d, testOrigin, "localhost"), testOrigin, cookie)
				if w.Code != 200 || !strings.Contains(w.Body.String(), second.account.ID) {
					t.Fatal("duplicate display names confused identity lookup")
				}
				if _, err := pool.ExecContext(ctx, "UPDATE passkey_credentials SET rp_id='other.example' WHERE account_id=$1", user.account.ID); err == nil {
					t.Fatal("RP foreign-key invariant missing")
				}
				// Move both rows together in a deferred-free order by inserting a separate RP account.
				other, otherKey := loginIdentity(t)
				if err := saveRegistration(ctx, pool, "other.example", registrationUser{ID: uuid.MustParse(other.account.ID), Handle: other.account.WebauthnUserHandle, DisplayName: other.account.DisplayName}, &other.credential); err != nil {
					t.Fatal(err)
				}
				options, cookie = beginLoginTest(t, handler, nil)
				if w := loginRequestTest(handler, "finish", assertionResponse(t, otherKey, other, options, 1, 0x1d, testOrigin, "localhost"), testOrigin, cookie); w.Code != 401 {
					t.Fatal("credential from another RP accepted")
				}
			case "failed verification":
				_, wrongKey := loginIdentity(t)
				w := loginRequestTest(handler, "finish", assertionResponse(t, wrongKey, user, options, 1, 0x1d, testOrigin, "localhost"), testOrigin, cookie)
				if w.Code != 401 || len(w.Result().Cookies()) != 0 {
					t.Fatal("invalid signature granted access")
				}
				var untouched bool
				if err := pool.QueryRowContext(ctx, "SELECT sign_count=0 AND last_used_at IS NULL FROM passkey_credentials").Scan(&untouched); err != nil || !untouched {
					t.Fatal("invalid signature changed metadata")
				}
			case "update failure", "commit failure":
				if scenario == "commit failure" {
					rejectCommit(t, pool, "UPDATE", "reject_login")
				} else {
					if _, err := pool.ExecContext(ctx, `CREATE TRIGGER reject_login BEFORE UPDATE ON passkey_credentials BEGIN SELECT RAISE(ABORT, 'private test failure'); END`); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if _, err := pool.ExecContext(ctx, "DROP TRIGGER reject_login"); err != nil {
							t.Error(err)
						}
					})
				}

				w := loginRequestTest(handler, "finish", body, testOrigin, cookie)
				if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "private") {
					t.Fatal("metadata failure granted access or leaked details")
				}
				var untouched bool
				if err := pool.QueryRowContext(ctx, "SELECT sign_count=0 AND last_used_at IS NULL FROM passkey_credentials").Scan(&untouched); err != nil || !untouched {
					t.Fatal("metadata was not rolled back")
				}
				if retry := loginRequestTest(handler, "finish", body, testOrigin, cookie); retry.Code != 400 {
					t.Fatal("failed update allowed challenge reuse")
				}
			case "session failure":
				store := h.sessions.Store
				h.sessions.Store = &failingSessionStore{Store: store, failCommit: true}
				w := loginRequestTest(handler, "finish", body, testOrigin, cookie)
				h.sessions.Store = store
				if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), `"account":`) {
					t.Fatal("failed session commit sent success")
				}
				var count int64
				if err := pool.QueryRowContext(ctx, "SELECT sign_count FROM passkey_credentials").Scan(&count); err != nil || count != 1 {
					t.Fatal("verified metadata was lost")
				}
				options, cookie = beginLoginTest(t, handler, cookie)
				if w := loginRequestTest(handler, "finish", assertionResponse(t, key, user, options, 2, 0x1d, testOrigin, "localhost"), testOrigin, cookie); w.Code != 200 {
					t.Fatal("could not retry with a fresh ceremony")
				}
			case "concurrent metadata":
				second, otherCookie := beginLoginTest(t, handler, nil)
				otherBody := assertionResponse(t, key, user, second, 2, 0x1d, testOrigin, "localhost")
				start := make(chan struct{})
				results := make(chan int, 2)
				var wg sync.WaitGroup
				for _, attempt := range []struct {
					body   string
					cookie *http.Cookie
				}{{body, cookie}, {otherBody, otherCookie}} {
					wg.Add(1)
					go func() {
						defer wg.Done()
						<-start
						results <- loginRequestTest(handler, "finish", attempt.body, testOrigin, attempt.cookie).Code
					}()
				}
				close(start)
				wg.Wait()
				close(results)
				for code := range results {
					if code != 200 {
						t.Fatalf("concurrent login failed: %d", code)
					}
				}
				var count int64
				if err := pool.QueryRowContext(ctx, "SELECT sign_count FROM passkey_credentials").Scan(&count); err != nil || count != 2 {
					t.Fatal("concurrent update lost the newer counter")
				}
			}
		})
	}
}
