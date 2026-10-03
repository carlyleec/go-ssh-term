// Package sqlite defines the connection policy shared by the app and its tests.
package sqlite

import (
	"context"
	"net/url"
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
