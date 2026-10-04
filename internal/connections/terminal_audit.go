package connections

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/google/uuid"
)

// An attempt keeps the authorized destination used for dialing, even after edits
// or deletion. Only fixed application codes enter failure records.
type terminalAudit struct {
	d       *Dialer
	row     queries.SavedConnection
	attempt string
	mu      sync.Mutex
	failure string
}

func (a *terminalAudit) fail(code string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failure == "" {
		a.failure = code
	}
}

func (a *terminalAudit) insert(ctx context.Context, q *queries.Queries, event, code string) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	return q.InsertConnectionAuditEvent(ctx, queries.InsertConnectionAuditEventParams{
		ID: id.String(), AccountID: a.row.AccountID, SavedConnectionID: a.row.ID,
		AttemptID: a.attempt, ConnectionName: a.row.Name, Host: a.row.Host,
		Port: a.row.Port, Username: a.row.Username, EventType: event,
		FailureCode: sql.NullString{String: code, Valid: code != ""}, OccurredAt: time.Now().UTC().UnixNano(),
	})
}

// Cleanup must persist after cancellation, but storage cannot hold teardown
// indefinitely. Failure and end are atomic; a storage outage leaves an open
// start record and a safe diagnostic rather than a fabricated successful end.
func (a *terminalAudit) finish() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.finishContext(ctx); err != nil {
		log.Printf("terminal audit completion could not be recorded: attempt=%s", a.attempt)
	}
}
func (a *terminalAudit) finishContext(ctx context.Context) error {
	tx, err := a.d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := a.d.q.WithTx(tx)
	a.mu.Lock()
	code := a.failure
	a.mu.Unlock()
	if code != "" {
		if err := a.insert(ctx, q, "failure", code); err != nil {
			return err
		}
	}
	if err := a.insert(ctx, q, "end", ""); err != nil {
		return err
	}
	return tx.Commit()
}

func auditSetupCode(err error) string {
	var safe *ConnectionErrorBody
	if errors.As(err, &safe) {
		switch safe.status {
		case 409:
			return "host_trust_rejected"
		case 503:
			return "storage_unavailable"
		case 504:
			return "setup_timeout"
		}
	}
	return "ssh_setup_failed"
}
