# Preserve account-access guarantees on SQLite

Status: Accepted; partially supersedes the Postgres-specific storage decisions in [004](004-auth-libraries.md), [005](005-account-storage.md), [007](007-registration-session-boundary.md), and [008](008-passkey-login.md). Their authentication and session guarantees remain accepted. Implementation is tracked in Slice 2.5.

## Context

Moving to [SQLite](../api/008-sqlite-storage.md) replaces pgx, Postgres column types,
and credential row locks. Registration, passkey metadata, and server sessions must
retain their existing correctness and failure boundaries.

## Decision

Keep go-webauthn and SCS. Select and pin a SQLite session adapter compatible with
the chosen driver and shared `sql.DB`; verify its schema, expiry representation,
context support, and cleanup lifecycle. Prefer an existing compatible adapter;
if it cannot meet these requirements, implement a small SCS context-aware store
and record that choice before proceeding. SCS's documented `sqlite3store` targets
mattn/go-sqlite3, so compatibility with modernc must not be assumed.

Use strict SQLite tables for application accounts and credentials, with canonical
UUID text IDs, BLOB handles/credential IDs/public keys/AAGUIDs, and INTEGER counters
and flags with the existing range checks. Encode transports as a JSON text array
and nested credential metadata as JSON text objects with validity/type checks.
Use an explicit UTC timestamp representation and test precision round trips;
the sessions table follows the verified adapter's storage contract.
Preserve nonblank, non-unique display names, RP-scoped handle/credential uniqueness,
one credential per account, composite account/RP foreign keys, and cascading
credential deletion. Enforce foreign keys on every connection.

Registration saves the account and full credential in one transaction. Login
uses an immediate transaction spanning RP/credential/handle lookup, local signature
verification, and counter/flag/last-use updates. This replaces `FOR UPDATE` with
SQLite's database-wide writer reservation. Preserve clone-warning and backup-state
behavior and generic verification errors. Map SQLite constraint and storage errors
to the existing safe API outcomes without exposing SQL or credential data.

Commit account creation or login metadata before replacing the anonymous session.
Release the transaction's connection before calling SCS. Preserve token rotation,
full absolute lifetime, explicit session saving before success, single-use pending
challenges, and passkey-login recovery after account persistence succeeds but
session creation fails. Logout still deletes only the current session before
clearing its cookie and invoking the existing lifecycle boundary as documented in
[010](010-logout-and-session-expiry.md).

## Consequences

All Slice 2 behavior remains required on the new database. Tests must exercise
concurrent credential updates, duplicate registration, transaction rollback,
constraint failures, session persistence failures, expiry, cleanup, and logout
isolation. Foreign-key and full metadata round trips are required, not inferred
from successful SQL generation. Registration and login session storage remain a
separate commit boundary even though they use the same database file.

Existing Postgres accounts and cookies do not carry over. Users register again;
new SQLite sessions and accounts must survive ordinary restarts and mode switches.

## References

- [SCS session stores and context-aware store interface](https://github.com/alexedwards/scs#configuring-the-session-store)
- [SQLite foreign-key enforcement](https://sqlite.org/foreignkeys.html)
