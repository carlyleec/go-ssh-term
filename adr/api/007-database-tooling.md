# Use sqlc, pgx, and dbmate for database access

Status: Superseded by [008 SQLite storage](008-sqlite-storage.md). The replacement is planned in Slice 2.5; this record describes the original implementation.

## Context

The application needs typed Go queries and an explicit, reviewable SQL migration
workflow while keeping feature code straightforward.

## Decision

Use sqlc to generate typed Go queries from SQL and pgx with a shared connection
pool at runtime. Keep migrations in `db/migrations` and query SQL in `db/queries`.
Commit generated Go alongside SQL changes. Feature handlers call generated queries
directly; do not add a repository abstraction without a concrete need.

Use dbmate for timestamped SQL migrations, run explicitly through pinned Docker
tooling. Use the migration files as sqlc's schema input and disable the redundant
schema dump. Apply migrations in strict order and do not edit applied migrations.

Require a database URL and verify connectivity and the dbmate migration ledger
before accepting HTTP requests. Embed migration filenames with the application to
check for pending or unknown versions. The server does not perform migrations.

## Consequences

Query and schema changes remain SQL, with generated types checked by Go. Developers
run migrations and regenerate queries explicitly. Database startup failures stop
the application with actionable errors without exposing connection credentials.
Migration history checks do not detect manual schema drift. Domain tables are
introduced by the slices that need them.

This supplements [the Postgres decision](002-postgres.md).
