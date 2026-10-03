// Package sqlite defines the connection policy shared by the app and its tests.
package sqlite

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const OperationTimeout = 10 * time.Second

// DSN applies connection-local settings whenever database/sql opens a connection.
// The caller validates the absolute path and chooses rw or rwc explicitly.
func DSN(path, mode string) string {
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"mode": {mode}, "_txlock": {"immediate"}, "_pragma": {
		"busy_timeout(5000)", "foreign_keys(ON)", "journal_mode(WAL)", "synchronous(FULL)",
	}}
	u.RawQuery = q.Encode()
	return u.String()
}

func WorkContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, OperationTimeout)
}

// ValidatePath disallows relative paths and SQLite URI/in-memory alternatives.
func ValidatePath(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsRune(path, 0) {
		return errors.New("DATABASE_PATH must be an absolute, clean filesystem path")
	}
	return nil
}
