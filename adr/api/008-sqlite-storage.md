# Use SQLite for local persistent storage

Status: Accepted; supersedes [002 Postgres](002-postgres.md) and [007 Database tooling](007-database-tooling.md). Lock-wait cancellation is amended by [009](009-sqlite-lock-wait-deadlines.md). Implementation is tracked in Slice 2.5.

## Context

The gateway runs as one local Go process. Its database stores accounts, session
state, configuration, and audit events, while terminal traffic stays outside the
database. An embedded relational database removes a service from the demo and
provides useful experience with explicit transaction and concurrency management.

## Decision

Use one SQLite file for application data and SCS sessions. Demo and development
share a named Docker volume mounted as a writable directory, including SQLite's
WAL and shared-memory files. Keep future SSH encryption material in a separate
volume. Support one app process and local filesystem storage; do not run demo and
development against the file simultaneously or mount it over a network filesystem.

Use `database/sql` with the CGO-free `modernc.org/sqlite` driver pinned at v1.60.1.
Keep sqlc v1.31.1 query generation and explicit dbmate v2.36.0 migrations. A
strict-table disposable-file check verified dbmate up/down/up, deterministic
sqlc generation, and binary/nanosecond round trips through generated queries
and the local SCS adapter described in [auth ADR 011](../auth/011-sqlite-account-persistence.md).
The active schema and queries use SQLite; sqlc SQLite support is beta.

Share a single `sql.DB` across application queries and session storage, with
`SetMaxOpenConns(1)` and `SetMaxIdleConns(1)`. This deliberately serializes database
operations, including reads, for the initial local workload. Do not introduce a
second read pool without evidence that it is needed. Use WAL, `synchronous=FULL`,
foreign-key enforcement, and a five-second busy timeout. Configure connection-local
settings for every new connection, not just a startup `Exec`; verify WAL at startup.
Use a ten-second context budget for request-driven database work (or an earlier
caller deadline), including pool acquisition and session operations. Pool waits
respect cancellation; calls inside SQLite may finish an external lock wait at
the five-second busy limit, as specified in [009](009-sqlite-lock-wait-deadlines.md).
Readiness retains its shorter probe context with the same lock-wait limitation.

Use immediate transactions for read/modify/write operations. Acquire the writer
reservation before reading state that will be updated. Keep all transaction queries
on the transaction handle; calling the pool or session store while holding its only
connection can deadlock. Commit or roll back before subsequent session operations.
Do not hold transactions across browser prompts, SSH dialing, or terminal I/O.
Do not replay an entire authentication ceremony automatically after a lock failure.
WAL permits reader/writer overlap across connections, not concurrent writers.

Start with a fresh SQLite database and re-register accounts. No Postgres data or
session import is included. Preserve the old Postgres volume. Retain its applied
migration files unchanged in an inactive legacy directory; create a separate SQLite
migration lineage and configure embedding, sqlc, and dbmate to use only that lineage.
Later migrations remain immutable once applied.

Keep migration-first startup: dbmate creates/migrates the file explicitly while
the app is stopped. The app opens an existing file and rejects missing, pending,
or unknown migration history before serving. Define one absolute database path
shared by app and migration tooling, validating configuration rather than accepting
an in-memory fallback. Ensure the non-root app can write both the file and directory;
migration tooling must not leave files it cannot write. Readiness checks actual
database access, and shutdown stops session cleanup before closing the database.

## Consequences

Deployment and integration tests no longer need a database server. Writes still
serialize; long transactions delay unrelated account/session requests. Locking,
pool waits, expiry, and cancellation need explicit regression tests. Use separate
files in `t.TempDir()` with production settings for integration tests, including a
second connection to create real contention; do not substitute shared in-memory
SQLite for file/WAL tests.

This decision changes storage, not authorization or account-access behavior. Multiple replicas, Postgres compatibility, and online data migration
are outside scope.

## References

- [SQLite isolation and immediate transactions](https://sqlite.org/isolation.html)
- [SQLite WAL](https://sqlite.org/wal.html)
- [Go connection management](https://go.dev/doc/database/manage-connections)
- [modernc SQLite driver](https://pkg.go.dev/modernc.org/sqlite)
- [sqlc database support](https://docs.sqlc.dev/en/latest/reference/language-support.html)
- [dbmate SQLite configuration](https://github.com/amacneil/dbmate#sqlite)
