# Use Postgres for persistent application data

Status: Superseded by [008 SQLite storage](008-sqlite-storage.md). The replacement is planned in Slice 2.5; this record describes the original implementation.

## Context

Accounts, credentials, saved connections, and host trust must survive container recreation and remain isolated by user.

## Decision

Use Postgres with migrations and persistent Docker storage for accounts, public passkey credentials, login sessions, encrypted SSH keys, saved connections, host trust, and audit events. Keep active SSH connections in Go memory. Use [sqlc, pgx, and dbmate](007-database-tooling.md) for queries, pooling, and migrations.

## Consequences

The app has one persistent database and requires migration and volume management. Database records cannot preserve running shells or network connections. A database backup alone cannot decrypt SSH keys without the separate application encryption key.
