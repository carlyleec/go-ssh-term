package database_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
)

func TestOpenAndReplacement(t *testing.T) {
	db, path := testdb.New(t)
	if db.Stats().MaxOpenConnections != 1 {
		t.Fatal("database must serialize operations")
	}
	for range 2 {
		if err := database.Check(t.Context(), db); err != nil {
			t.Fatal(err)
		}
		for pragma, want := range map[string]string{"journal_mode": "wal", "foreign_keys": "1", "synchronous": "2", "busy_timeout": "5000"} {
			var got string
			if err := db.QueryRowContext(t.Context(), "PRAGMA "+pragma).Scan(&got); err != nil || got != want {
				t.Fatalf("%s = %s, %v", pragma, got, err)
			}
		}
		db.SetMaxIdleConns(0)
		db.SetMaxIdleConns(1)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := database.Check(t.Context(), db); err == nil {
		t.Fatal("closed database ready")
	}
	reopened, err := database.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := database.Check(t.Context(), reopened); err != nil {
		t.Fatal(err)
	}
}

func TestStorageFailures(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"", ":memory:", "relative.db", "file:/tmp/test.db", filepath.Join(dir, "missing.db"), dir} {
		if db, err := database.Open(t.Context(), path); err == nil {
			db.Close()
			t.Fatalf("accepted %q", path)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "missing.db")); !os.IsNotExist(err) {
		t.Fatal("startup created a missing database")
	}
	path := filepath.Join(dir, "unmigrated.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if db, err := database.Open(t.Context(), path); err == nil {
		db.Close()
		t.Fatal("unmigrated database accepted")
	} else if !strings.Contains(err.Error(), "make migrate") {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a SQLite database"), 0600); err != nil {
		t.Fatal(err)
	}
	if db, err := database.Open(t.Context(), path); err == nil {
		db.Close()
		t.Fatal("corrupt database accepted")
	}
}

func TestUnwritableStorage(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks require a non-root process")
	}
	db, path := testdb.New(t)
	db.Close()
	for _, target := range []string{path, filepath.Dir(path)} {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(target, 0400); err != nil {
			t.Fatal(err)
		}
		got, openErr := database.Open(t.Context(), path)
		restoreErr := os.Chmod(target, info.Mode().Perm())
		if restoreErr != nil {
			t.Fatal(restoreErr)
		}
		if openErr == nil {
			got.Close()
			t.Fatalf("accepted unwritable %s", target)
		}
	}
}

func TestMigrationFailuresAndPoolCancellation(t *testing.T) {
	db, path := testdb.New(t)
	for _, tc := range []struct{ statement, want string }{
		{"DELETE FROM schema_migrations", "pending"},
		{"INSERT INTO schema_migrations VALUES ('20261003000300'), ('20261003000100')", "unknown"},
	} {
		if _, err := db.ExecContext(t.Context(), tc.statement); err != nil {
			t.Fatal(err)
		}
		other, err := database.Open(t.Context(), path)
		if err == nil {
			other.Close()
			t.Fatal("bad history accepted")
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatal(err)
		}
	}
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	if err := database.Check(ctx, db); err == nil {
		t.Fatal("pool wait ignored cancellation")
	}
	conn.Close()
	if err := database.Check(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	// No-row errors retain database/sql semantics.
	var ignored int
	if err := db.QueryRow("SELECT 1 WHERE 0").Scan(&ignored); err != sql.ErrNoRows {
		t.Fatal(err)
	}
}
