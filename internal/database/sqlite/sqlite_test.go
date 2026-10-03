package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	driver "modernc.org/sqlite"
	"modernc.org/sqlite/lib"
)

func rawOpen(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", sqlite.DSN(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	return db
}

func open(t *testing.T, path string) *sql.DB {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return testdb.Create(t, path)
	}
	db, err := database.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestConnectionPolicySurvivesReplacement(t *testing.T) {
	db := open(t, filepath.Join(t.TempDir(), "settings ?#.db"))
	for i := 0; i < 2; i++ {
		for pragma, want := range map[string]string{"journal_mode": "wal", "foreign_keys": "1", "synchronous": "2", "busy_timeout": "5000"} {
			var got string
			if err := db.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("%s = %s, want %s", pragma, got, want)
			}
		}
		db.SetMaxIdleConns(0)
		db.SetMaxIdleConns(1)
	}
}

func TestImmediateTransactionExternalLockTimeoutAndRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock.db")
	first, second := open(t, path), open(t, path)
	tx, err := first.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// No write has occurred: the transaction itself must reserve the writer.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	other, err := second.BeginTx(ctx, nil)
	if other != nil {
		other.Rollback()
		t.Fatal("second writer acquired reserved database")
	}
	var sqliteErr *driver.Error
	if !errors.Is(err, context.DeadlineExceeded) && !(errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_BUSY) {
		t.Fatalf("begin error = %v", err)
	}
	// SQLite's busy handler can finish its five-second wait after cancellation.
	// Allow scheduling overhead without accepting an unbounded lock wait.
	if elapsed := time.Since(started); elapsed > 7*time.Second {
		t.Fatalf("external lock wait took %s", elapsed)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	other, err = second.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionPoolWaitCancellation(t *testing.T) {
	db := open(t, filepath.Join(t.TempDir(), "pool.db"))
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	other, err := db.BeginTx(ctx, nil)
	if other != nil {
		other.Rollback()
		t.Fatal("transaction acquired occupied pool connection")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pool wait error = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("pool wait ignored deadline")
	}
}

func TestRunningQueryCancellation(t *testing.T) {
	db := open(t, filepath.Join(t.TempDir(), "query.db"))
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := db.ExecContext(ctx, `WITH RECURSIVE n(x) AS
		(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x < 1000000000)
		SELECT sum(x) FROM n`)
	if err == nil || ctx.Err() == nil {
		t.Fatalf("running query did not cancel: %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("running query ignored deadline")
	}
	var one int
	if err := db.QueryRowContext(t.Context(), "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("connection did not recover: %d, %v", one, err)
	}
}
