// Package sqlitestore implements SCS storage on the application's shared SQLite pool.
package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
)

var _ scs.CtxStore = (*Store)(nil)

// Store requires sessions(token TEXT PRIMARY KEY, data BLOB, expiry INTEGER).
// Expiry is UTC Unix nanoseconds; it is never rounded to SQLite date precision.
type Store struct {
	db     *sql.DB
	cancel context.CancelFunc
	done   chan struct{}
	stop   sync.Once
}

// New starts periodic cleanup. A zero interval disables cleanup for short-lived stores.
func New(db *sql.DB, interval time.Duration) *Store {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Store{db: db, cancel: cancel, done: make(chan struct{})}
	if interval <= 0 {
		close(s.done)
		return s
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.deleteExpired(ctx); err != nil && ctx.Err() == nil {
					slog.Error("could not clean up expired sessions")
				}
			}
		}
	}()
	return s
}

// StopCleanup cancels and joins cleanup before the owner closes the shared pool.
func (s *Store) StopCleanup() {
	s.stop.Do(s.cancel)
	<-s.done
}

func (s *Store) Find(token string) ([]byte, bool, error) {
	return s.FindCtx(context.Background(), token)
}

func (s *Store) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	var data []byte
	var expiry int64
	err := s.db.QueryRowContext(ctx, "SELECT data, expiry FROM sessions WHERE token = ?", token).Scan(&data, &expiry)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	// Evaluate expiry after the query: acquiring the shared connection can wait.
	if err == nil && expiry <= time.Now().UnixNano() {
		return nil, false, nil
	}
	return data, err == nil, err
}

func (s *Store) Commit(token string, data []byte, expiry time.Time) error {
	return s.CommitCtx(context.Background(), token, data, expiry)
}

func (s *Store) CommitCtx(ctx context.Context, token string, data []byte, expiry time.Time) error {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token, data, expiry) VALUES (?, ?, ?)
		ON CONFLICT(token) DO UPDATE SET data = excluded.data, expiry = excluded.expiry`, token, data, expiry.UnixNano())
	return err
}

func (s *Store) Delete(token string) error {
	return s.DeleteCtx(context.Background(), token)
}

func (s *Store) DeleteCtx(ctx context.Context, token string) error {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (s *Store) deleteExpired(ctx context.Context) error {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expiry <= ?", time.Now().UnixNano())
	return err
}
