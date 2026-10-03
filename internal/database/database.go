package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/carlyleec/go-ssh-term/db/legacy/migrations"
	"github.com/carlyleec/go-ssh-term/internal/database/queries"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open connects and verifies the migration history before the server accepts requests.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL for Postgres")
	}
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("could not initialize Postgres connection pool")
	}
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := queries.New(pool).CheckConnection(startupCtx); err != nil {
		pool.Close()
		return nil, errors.New("could not connect to Postgres; check DATABASE_URL, credentials, and database readiness")
	}
	if err := checkMigrations(startupCtx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func checkMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// This table belongs to dbmate, not the application's sqlc schema.
	rows, err := pool.Query(ctx, "SELECT version FROM public.schema_migrations ORDER BY version")
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return errors.New("database migrations are not initialized; run make migrate")
		}
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
	return verifyVersions(applied)
}

func verifyVersions(applied map[string]bool) error {
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
