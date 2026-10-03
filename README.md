# Browser SSH Gateway

A Go and React browser SSH gateway in development. The frontend has a landing
page at `/` and placeholder pages at `/login` and `/connections`. Account access
and SSH features are not implemented; the connections preview is currently public.

## Demo with Docker

Docker builds the React assets and Go executable; no host Go or Bun installation
is needed. From a fresh checkout, start Postgres and explicitly apply migrations
before starting the application:

```sh
docker compose up -d --wait postgres
docker compose run --rm --no-deps dbmate
docker compose up --build -d --wait
```

Open http://127.0.0.1:8080. The image contains the compiled frontend and a Go
server running as a non-root user. It has no source mounts or development servers.
The demo fixes the container HTTP address at `:8080` and publishes it only on
IPv4 loopback. `BROWSER_ORIGIN` defaults to `http://127.0.0.1:8080`; if an existing
`.env` sets the Vite origin on port 5173, remove that override or change it for
the demo. Other configuration is described below.

The server checks migration history before accepting requests and never runs
migrations itself. Repeat the migration command after pulling schema changes.
Postgres readiness alone does not mean migrations have been applied.

```sh
docker compose ps
docker compose logs -f app postgres
docker compose run --rm --no-deps dbmate status
docker compose down
```

Ordinary shutdown preserves the `postgres-data` volume. Demo and development
use the same Compose project and database volume when run from this directory;
stop one mode before starting the other (`make down` for development,
`docker compose down` for the demo). Do not add `--volumes` when switching modes.

`GET /api/readyz` returns 204 when a database ping succeeds and the frontend
entry point is readable, or 503 otherwise. The database probe has a two-second
deadline. Docker probes every five seconds and marks the app unhealthy after
three failures; it does not automatically restart an unhealthy container.
Migration history is checked at startup, not on each readiness request.

## Development with Docker

With Docker running and Make installed, start the Go and frontend development servers:

```sh
make migrate
make up
```

Open http://127.0.0.1:5173 for the React frontend. The Compose port is bound to
IPv4 loopback; using this address avoids reaching a different service if
`localhost` resolves to IPv6 (`::1`). Vite updates the browser when
you edit React components or styles. The Go server serves the compiled frontend
at http://127.0.0.1:8080 after `make build`. API proxying through Vite remains
a later task.

Air rebuilds and restarts Go when Go source or SQL migrations change. Go build errors appear in
the container logs and stop the previous server until the build succeeds.
Frontend files are excluded from Air's watcher. Both watchers poll mounted source
to support Docker Desktop reliably.

The containers pin Go 1.27.0 and Bun 1.4.2. Docker alone is enough to run the
application; local editor tooling uses matching versions from `.tool-versions`.
With asdf and its `golang` and `bun` plugins installed, prepare the host tools and
dependencies:

```sh
make setup
```

This installs the pinned runtimes, downloads Go modules for local `gopls`, and
installs macOS/host frontend dependencies into `frontend/node_modules`. Install
`gopls` through your editor's Go tooling if it is not already available. Ensure
your editor uses the asdf shims on PATH and does not override `GOROOT`; the Go
toolchain determines its own root. Restart the editor after changing its environment.

Air is pinned in `go.mod`. Frontend dependencies use exact versions in
`frontend/package.json`, with transitive versions recorded in `frontend/bun.lock`.
Both host setup and container startup use a frozen lockfile. Rerun `make setup`
after pulling dependency changes or adding dependencies through Docker.

Host and container installations are separate: your editor sees local
`frontend/node_modules`, while Docker mounts a named volume at `/app/node_modules`
for Linux dependencies. Both use the same manifest and lockfile. This avoids
mixing native macOS and Linux packages. The local directory is ignored by Git.
Docker also keeps Go dependencies, build caches, Air binaries, Bun's download
cache, and Vite build output in volumes. These are disposable build artifacts.

Run `make help` (or just `make`) to list the available commands. Targets wrap
`docker compose -f compose.dev.yaml`; the Compose file can still be used directly.
With `make up` running in one terminal, use another terminal for:

```sh
make logs       # Follow Go, frontend, and Postgres logs
make ps         # Show service status
make check      # Go tests, Biome checks, and TypeScript checks
make test       # Go package tests only
make lint       # Frontend lint rules only
make typecheck  # Frontend TypeScript checks only
make format     # Write frontend formatting changes
make fix        # Apply safe Biome fixes, including import ordering
```

`make check` does not edit source files. Biome understands Tailwind directives and
explicitly excludes dependency directories, build output, and the Bun lockfile.
Tests, checks, formatting, and fixes require their services to be running.

Build production frontend assets with:

```sh
make build
```

This runs a frozen-lockfile install, type checking, and a Vite build in a temporary
container, so it also works when development services are stopped. Output is stored
in the frontend build volume at `/app/dist` inside the container.

The Go container mounts that same volume read-only at `/app/frontend/dist`.
After `make build` and `make up`, open http://127.0.0.1:8080 to use the compiled
frontend through Go. Rebuild and refresh to see frontend changes there; Vite on
port 5173 remains the hot-update workflow. No Go restart is needed after a build.
For a host-run Go server, build the frontend locally and run Go from the repository
root so `frontend/dist` resolves correctly.

Direct navigation to `/login`, `/connections`, and other extensionless page URLs
serves the React entry point. Missing `/api` endpoints, `/assets` files, and URLs
with file extensions return 404. Only GET and HEAD are supported for frontend
requests, and directories are not listed. An absent frontend build returns 503
with build instructions. The demo image builds and includes these assets itself.


To add a frontend dependency, use Bun in the container and review both the
manifest and lockfile changes:

```sh
docker compose -f compose.dev.yaml exec frontend bun add --exact PACKAGE
docker compose -f compose.dev.yaml exec frontend bun add --dev --exact PACKAGE
```

After pulling dependency changes, recreate the frontend service to install from
the updated lockfile:

```sh
make frontend-restart
```

Stop the services while preserving volumes:

```sh
make down
```

This standalone development setup includes Go, the frontend, and persistent
Postgres. It reuses the Postgres and dbmate definitions from `compose.yaml`.
Integration of Air and Vite into a development override remains a later Slice 1 task.

## Frontend routes

TanStack Router reads route files from `frontend/src/routes`. `__root.tsx` defines
the shared layout; `index.tsx`, `login.tsx`, and `connections.tsx` define the pages.
Each route file contains its page component.

Vite regenerates `src/routetree.gen.ts` as routes change. Type checking and builds
also generate it first. Keep this generated file in source control alongside route
changes; do not edit it manually. To regenerate it explicitly on the host:

```sh
cd frontend
bun run routes
```

## Server configuration

Copy `.env.example` to `.env` to customize Docker settings. `.env` is
ignored by Git; do not commit credentials. Compose passes the values into the Go
container. The Go executable itself reads process environment variables and does
not load `.env` files. Export the variables explicitly when running Go locally.

| Setting | Go default | Meaning |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Listen address in `host:port` form, including `:port` or `[IPv6]:port`. |
| `DATABASE_URL` | Required | Postgres connection URL; Compose supplies the local development URL. |
| `BROWSER_ORIGIN` | `http://127.0.0.1:8080` | Exact browser origin, without a trailing slash, path, query, or credentials. |
| `SHUTDOWN_TIMEOUT` | `5s` | Positive Go duration for draining HTTP requests on SIGINT or SIGTERM. |

Development Compose defaults `BROWSER_ORIGIN` to `http://127.0.0.1:5173` for Vite;
demo Compose defaults to port 8080. Leave the variable unset to keep these
mode-specific defaults. Use the 8080 origin when testing Go-served assets in
development. Origin validation here checks config
syntax only; request-origin enforcement and WebAuthn are implemented with auth.
Database URLs are syntax-checked without logging their contents. Startup requires
a reachable database with the expected migration history before opening the HTTP
listener. An explicitly empty `DATABASE_URL` fails; if an older `.env` contains
an empty value, replace it with the development URL in `.env.example`.

In development, changing `HTTP_ADDR` does not change Compose's published port automatically. Keep
its container port mapping in sync, and use an unspecified host (`:8080`) to
accept traffic forwarded into the container.

Normal HTTP shutdown waits up to `SHUTDOWN_TIMEOUT`, then closes remaining HTTP
connections and exits with an error if the deadline expires. Future SSH/WebSocket
sessions need their own cleanup; HTTP shutdown does not close hijacked connections.
Air sends an interrupt and allows 10 seconds before killing Go. Compose's
`APP_STOP_GRACE_PERIOD` defaults to 15 seconds. If increasing the shutdown timeout,
also increase Air's `kill_delay` and Docker's grace period to leave enough time.

Environment changes require recreating the Go service:

```sh
docker compose -f compose.dev.yaml up -d --force-recreate app
```

## Database and queries

Both modes use Postgres 18.1 with a named `postgres-data` volume. Ordinary
container recreation and `make down` preserve it. Unlike build caches, this volume
contains application data. Postgres is reachable as `postgres:5432` inside Compose
and has no published host port. The fixed `gateway` / `gateway-local-only`
credentials are for this disposable local lab only. To run Go on the host, provide
a reachable Postgres URL; the Compose hostname does not resolve on the host.

Run `make migrate` before the first startup and whenever you pull new migrations.
It starts Postgres, waits for readiness, and runs pinned dbmate with strict ordering.
Repeated runs apply only pending migrations. `make migrate-status` shows the ledger.
The Go process checks migration versions but never applies migrations itself.
The pool allows up to ten connections, uses a five-second startup deadline, and
closes after HTTP shutdown. Missing, pending, or unknown migration versions prevent
startup. This checks migration history, not manual schema drift.

SQL migrations in `db/migrations` are the schema source for both dbmate and sqlc;
we disable dbmate's separate schema dump. The initial migration establishes the
ledger without introducing feature tables. Add those tables with their slices.
Create a migration using:

```sh
docker compose -f compose.dev.yaml run --rm --no-deps -T dbmate new create_accounts
```

Write SQL under the generated `-- migrate:up` and `-- migrate:down` markers. Keep
applied migrations immutable; add a new timestamped migration for subsequent
changes. Apply it with `make migrate`. Air watches SQL files, but if it previously
stopped because a migration was pending, restart Go after applying it:

```sh
docker compose -f compose.dev.yaml restart app
```

Write named queries in `db/queries` and run `make generate`. sqlc generates typed
Go methods for pgx in `internal/database/queries`. Review and commit generated Go
with its SQL changes; do not edit it manually. Feature handlers can use these
methods directly without an additional repository layer.

`make test` runs package tests. For database integration checks, run:

```sh
docker compose -f compose.dev.yaml run --rm --no-deps -T \
  -e 'TEST_DATABASE_URL=postgres://gateway:gateway-local-only@postgres:5432/gateway?sslmode=disable' \
  app go test -race ./...
```

Start Postgres first (`make migrate`). These checks create and remove uniquely
named test databases; the test role needs database-creation privileges. Without
`TEST_DATABASE_URL`, database integration checks are skipped.
