package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestVersions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		applied map[string]bool
		want    string
	}{
		{"current", map[string]bool{"20261003000100": true}, ""},
		{"pending", map[string]bool{}, "pending"},
		{"unknown", map[string]bool{"20261003000100": true, "20990101000000": true}, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyVersions(tc.applied)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestInvalidURLDoesNotLeakCredentials(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user:secret%zz@localhost/db")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("expected redacted configuration error")
	}
}

// TEST_DATABASE_URL must allow creating disposable databases for integration checks.
func TestPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run Postgres integration checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("gateway_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	testURL := u.String()
	if pool, err := Open(ctx, testURL); err == nil {
		pool.Close()
		t.Fatal("unmigrated database accepted")
	}
	raw, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(ctx, "CREATE TABLE public.schema_migrations (version varchar(255) PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if pool, err := Open(ctx, testURL); err == nil {
		pool.Close()
		t.Fatal("pending migration accepted")
	}
	if _, err := raw.Exec(ctx, "INSERT INTO public.schema_migrations VALUES ('20261003000100')"); err != nil {
		t.Fatal(err)
	}
	pool, err := Open(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	if _, err := raw.Exec(ctx, "INSERT INTO public.schema_migrations VALUES ('20990101000000')"); err != nil {
		t.Fatal(err)
	}
	if pool, err := Open(ctx, testURL); err == nil {
		pool.Close()
		t.Fatal("newer database accepted")
	}
	u.Path = "/nonexistent_" + name
	if pool, err := Open(ctx, u.String()); err == nil {
		pool.Close()
		t.Fatal("nonexistent database accepted")
	}
	u.Path = "/" + name
	u.User = url.UserPassword("gateway", "deliberately-wrong-secret")
	if pool, err := Open(ctx, u.String()); err == nil {
		pool.Close()
		t.Fatal("wrong credentials accepted")
	} else if strings.Contains(err.Error(), "deliberately-wrong-secret") {
		t.Fatal("credentials leaked")
	}
}
