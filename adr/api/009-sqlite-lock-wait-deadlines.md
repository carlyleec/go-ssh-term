# Separate pool cancellation from SQLite lock waits

Status: Accepted; supersedes the lock-wait cancellation requirement in [008](008-sqlite-storage.md).

## Context

With modernc.org/sqlite v1.60.1, canceling a context during `BEGIN IMMEDIATE`
blocked by another connection does not promptly interrupt SQLite's busy handler.
The call can return `SQLITE_BUSY` after the configured five-second busy timeout.
The shared one-connection Go pool does respect cancellation while acquiring a
connection. Normal application requests queue there, rather than competing for
SQLite writer locks.

## Decision

Keep modernc.org/sqlite and the connection policy in 008. Give request-driven
database work a ten-second context budget, preserving earlier caller deadlines.
Require prompt cancellation while waiting in the Go pool. For a call already
inside SQLite, allow an external lock wait to finish at the five-second busy
limit even if its context expires sooner. Context deadlines are cancellation
requests, not hard wall-clock guarantees for SQLite calls or filesystem I/O.
Readiness keeps its shorter context but has the same external-lock limitation.

Run migrations with the app stopped and support only one application process.
Do not add retries or background goroutines that return early while database
work continues. A lock failure returns a safe error; it does not replay a
transaction or authentication ceremony. Close or roll back transactions before
subsequent session operations.

Test pool deadline cancellation separately from external lock timeout and
recovery. Accept a busy or cancellation error after an externally blocked call,
require bounded completion with scheduling tolerance, and verify the connection
can be reused after the lock is released. Do not require a particular error
string or an exact elapsed time.

## Consequences

An external database tool can delay a canceled request by the remaining busy
wait. This is acceptable for the single-process local gateway and avoids custom
retry/driver machinery. It does not weaken account/session atomicity or authorize
success responses on storage failures. Cancellation of CPU-bound statements and
ordinary pool waits remains useful and is tested separately.
