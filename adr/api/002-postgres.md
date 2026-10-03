# Use Postgres for persistent application data

Status: Accepted

## Context

Accounts, credentials, saved connections, and host trust must survive container recreation and remain isolated by user.

## Decision

Use Postgres with migrations and persistent Docker storage for accounts, public passkey credentials, login sessions, encrypted SSH keys, saved connections, host trust, and audit events. Keep active SSH connections in Go memory.

## Consequences

The app has one persistent database and requires migration and volume management. Database records cannot preserve running shells or network connections. A database backup alone cannot decrypt SSH keys without the separate application encryption key.
