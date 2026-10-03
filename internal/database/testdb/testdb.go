// Package testdb creates isolated file databases with production migrations and settings.
package testdb

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlyleec/go-ssh-term/db/migrations"
	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
)

func New(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.db")
	raw, err := sql.Open("sqlite", sqlite.DSN(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	if _, err := raw.ExecContext(t.Context(), "CREATE TABLE schema_migrations (version TEXT PRIMARY KEY NOT NULL)"); err != nil {
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
		if _, err := raw.ExecContext(t.Context(), up); err != nil {
			t.Fatal(err)
		}
		version, _, _ := strings.Cut(file, "_")
		if _, err := raw.ExecContext(t.Context(), "INSERT INTO schema_migrations VALUES (?)", version); err != nil {
			t.Fatal(err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}
