package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/carlyleec/go-ssh-term/db/migrations"
)

// CheckMigrations accepts only the active SQLite lineage, never legacy Postgres versions.
func CheckMigrations(ctx context.Context, db *sql.DB) error {
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'schema_migrations'").Scan(&exists); err != nil {
		return errors.New("cannot read database migration history; check permissions and run make migrate-status")
	}
	if exists == 0 {
		return errors.New("database migrations are not initialized; run make migrate")
	}
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return errors.New("cannot read database migration history; check permissions and run make migrate-status")
	}
	defer rows.Close()
	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return errors.New("cannot read database migration version")
		}
		applied[version] = true
	}
	if rows.Err() != nil {
		return errors.New("cannot read database migration history")
	}
	files, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		return err
	}
	expected := make(map[string]bool)
	for _, file := range files {
		version, _, _ := strings.Cut(file, "_")
		expected[version] = true
		if !applied[version] {
			return fmt.Errorf("database migration %s is pending; run make migrate", version)
		}
	}
	for version := range applied {
		if !expected[version] {
			return errors.New("database has migrations unknown to this build; use matching application code")
		}
	}
	return nil
}
