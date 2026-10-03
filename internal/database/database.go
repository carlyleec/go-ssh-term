package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
)

// Open requires an existing, migrated file and writable directory for WAL journals.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := sqlite.ValidatePath(path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.New("database file is missing or inaccessible; check DATABASE_PATH and run make migrate")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("DATABASE_PATH must identify a regular database file")
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, errors.New("database file is not writable; check storage permissions")
	}
	if err := file.Close(); err != nil {
		return nil, errors.New("cannot close database file")
	}
	probe, err := os.CreateTemp(filepath.Dir(path), ".gateway-write-probe-*")
	if err != nil {
		return nil, errors.New("database directory is not writable; check storage permissions for SQLite journals")
	}
	closeErr := probe.Close()
	removeErr := os.Remove(probe.Name())
	if closeErr != nil || removeErr != nil {
		return nil, errors.New("cannot manage files in database directory; check storage permissions")
	}
	db, err := sql.Open("sqlite", sqlite.DSN(path, "rw"))
	if err != nil {
		return nil, errors.New("could not initialize SQLite database")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	fail := func(err error) (*sql.DB, error) { db.Close(); return nil, err }
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		return fail(errors.New("cannot open SQLite in WAL mode; check DATABASE_PATH, file and directory permissions"))
	}
	if err := sqlite.CheckMigrations(ctx, db); err != nil {
		return fail(err)
	}
	return db, nil
}

// Check reads database pages rather than merely checking the driver connection.
func Check(ctx context.Context, db *sql.DB) error {
	var tables int
	return db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema").Scan(&tables)
}
