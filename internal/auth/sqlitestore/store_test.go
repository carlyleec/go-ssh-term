package sqlitestore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
)

func newStore(t *testing.T, interval time.Duration) (*Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", sqlite.DSN(filepath.Join(t.TempDir(), "sessions.db"), "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE sessions (token TEXT PRIMARY KEY NOT NULL, data BLOB NOT NULL, expiry INTEGER NOT NULL) STRICT;
	CREATE INDEX sessions_expiry_idx ON sessions(expiry)`)
	if err != nil {
		t.Fatal(err)
	}
	s := New(db, interval)
	t.Cleanup(s.StopCleanup)
	return s, db
}

func TestSessionRoundTrip(t *testing.T) {
	s, db := newStore(t, 0)
	for _, data := range [][]byte{{0, 255, 128, 1}, {3, 0, 9}} {
		expiry := time.Now().Add(time.Hour).Truncate(time.Second).Add(123456789 * time.Nanosecond)
		if err := s.CommitCtx(t.Context(), "token", data, expiry); err != nil {
			t.Fatal(err)
		}
		got, found, err := s.FindCtx(t.Context(), "token")
		if err != nil || !found || !bytes.Equal(got, data) {
			t.Fatalf("find = %v, %v, %v", got, found, err)
		}
		var stored int64
		if err := db.QueryRow("SELECT expiry FROM sessions WHERE token = ?", "token").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if !time.Unix(0, stored).Equal(expiry) {
			t.Fatalf("expiry precision lost: %d", stored)
		}
	}
	if err := s.Commit("expired", []byte{1}, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.Find("expired"); err != nil || found {
		t.Fatalf("expired find = %v, %v", found, err)
	}
	if err := s.deleteExpired(t.Context()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sessions").Scan(&count); err != nil || count != 1 {
		t.Fatalf("cleanup count = %d, %v", count, err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Delete("token"); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, err := s.Find("token"); err != nil || found {
		t.Fatalf("deleted find = %v, %v", found, err)
	}
}

func TestPoolWaitRespectsContext(t *testing.T) {
	s, db := newStore(t, 0)
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, call := range []func(context.Context) error{
		func(ctx context.Context) error { _, _, err := s.FindCtx(ctx, "token"); return err },
		func(ctx context.Context) error {
			return s.CommitCtx(ctx, "token", []byte{1}, time.Now().Add(time.Hour))
		},
		func(ctx context.Context) error { return s.DeleteCtx(ctx, "token") },
	} {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		started := time.Now()
		err := call(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("pool wait error = %v", err)
		}
		if time.Since(started) > time.Second {
			t.Fatal("pool wait ignored deadline")
		}
	}
}

func TestStopCleanupJoinsAndIsIdempotent(t *testing.T) {
	s, db := newStore(t, time.Millisecond)
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(time.Second)
	for db.Stats().WaitCount == 0 {
		if time.Now().After(deadline) {
			t.Fatal("cleanup did not attempt database access")
		}
		time.Sleep(time.Millisecond)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Go(s.StopCleanup)
	}
	wg.Wait()
	select {
	case <-s.done:
	default:
		t.Fatal("cleanup still running")
	}
}

func TestExpiryCheckedAfterPoolWait(t *testing.T) {
	s, db := newStore(t, 0)
	if err := s.Commit("token", []byte{1}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	type result struct {
		found bool
		err   error
	}
	done := make(chan result, 1)
	go func() {
		_, found, err := s.FindCtx(t.Context(), "token")
		done <- result{found, err}
	}()
	deadline := time.Now().Add(time.Second)
	for db.Stats().WaitCount == 0 {
		if time.Now().After(deadline) {
			t.Fatal("lookup did not wait for the pool")
		}
		time.Sleep(time.Millisecond)
	}
	// Expire after the lookup started but before it can obtain a connection.
	if _, err := conn.ExecContext(t.Context(), "UPDATE sessions SET expiry = ?", time.Now().UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.err != nil || got.found {
			t.Fatalf("expired queued lookup = %v, %v", got.found, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("lookup did not complete after pool release")
	}
}
