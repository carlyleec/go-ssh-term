# Store connection configurations, endpoint trust, and audit snapshots separately

Status: Accepted

## Context

Saved configurations can change or disappear while a shell remains active.
Trust belongs to an account and SSH endpoint, and audit history must retain the
destination actually used. See [connection lifecycle](004-connection-lifecycle.md),
[host verification](../auth/003-host-verification.md), and
[audit events](006-audit-events.md).

## Decision

Use three strict SQLite tables with application-supplied UTC Unix nanoseconds:

- `saved_connections` stores account ownership, name, host, port, username,
  SSH key reference, and creation/update times. A composite foreign key enforces
  same-account key ownership and prevents deleting a referenced key. Account
  deletion cascades through both keys and configurations. Names need not be unique.
- `host_trust` stores one approved SSH wire-format public key per account,
  host, and port, independently of configurations or usernames. Derive the
  fingerprint from the public key. Endpoint normalization belongs to the API and
  SSH lookup paths and must agree before trust is used.
- `connection_audit_events` stores start, end, and failure events with an account,
  attempt UUID, historical saved-connection UUID, and a destination snapshot
  (name, host, port, username). Historical IDs deliberately have no connection
  foreign key, allowing final events after configuration deletion. Event writers
  must obtain ownership and snapshots from the authorized connection attempt.
  Failure events require a short application-owned code; later lifecycle code
  must select safe codes rather than persist raw errors. No session tokens,
  private keys, or terminal data belong in these records.

All three tables cascade on account deletion. Audit events survive configuration
edits/deletion, not account deletion. Live handles remain in memory, as already
decided. Jump references remain a later extension.

## Consequences

Database constraints protect key references, but do not replace account-scoped
queries or authorize audit writes. Trust changes need explicit application flows;
inserting a second key for the same endpoint fails. The schema does not implement
connection APIs, host approval, or audit event emission by itself.
