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

Open http://localhost:8080. The image contains the compiled frontend and a Go
server running as a non-root user. It has no source mounts or development servers.
The demo fixes the container HTTP address at `:8080` and publishes it only on
IPv4 loopback. `BROWSER_ORIGIN` defaults to `http://localhost:8080`; if an existing
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

Open http://localhost:5173 for development pages, API requests, and future terminal
WebSockets. Compose publishes ports only on IPv4 loopback. Use the `localhost`
hostname consistently because passkeys are scoped to the relying-party domain.
If another service listens on IPv6 localhost at these ports, stop it or choose
an unused port rather than switching the browser to an IP address. Vite updates
the browser when you edit React components or styles. Vite forwards `/api` and `/api/...`, including
WebSocket upgrades, to `http://app:8080` on the Compose network without changing
the path, browser Host, or Origin. Other paths stay with Vite, including its hot
update connection. Use relative `/api/...` URLs for HTTP and derive WebSocket
URLs from the browser's current host and scheme.

Keep `BROWSER_ORIGIN=http://localhost:5173` in development (the default).
The proxy preserves origin headers for backend validation. Registration
endpoints enforce the configured origin; protection for other account and
terminal routes is added alongside those endpoints. No permissive CORS setting is needed.
Go's port 8080 remains available for direct debugging and compiled-asset checks
after `make build`; use port 5173 for the development workflow.

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
`docker compose -f compose.yaml -f compose.dev.yaml`; this combined command can
also be used directly.
With `make up` running in one terminal, use another terminal for:

```sh
make logs       # Follow Go, frontend, and Postgres logs
make ps         # Show service status
```

See [Tests and checks](#tests-and-checks) for package tests, database integration
checks, frontend validation, and formatting commands.

Build production frontend assets with:

```sh
make build
```

This runs a frozen-lockfile install, type checking, and a Vite build in a temporary
container, so it also works when development services are stopped. Output is stored
in the frontend build volume at `/app/dist` inside the container.

The Go container mounts that same volume read-only at `/app/frontend/dist`.
After `make build` and `make up`, open http://localhost:8080 to use the compiled
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
docker compose -f compose.yaml -f compose.dev.yaml exec frontend bun add --exact PACKAGE
docker compose -f compose.yaml -f compose.dev.yaml exec frontend bun add --dev --exact PACKAGE
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

Development layers `compose.dev.yaml` over `compose.yaml`. The base owns
Postgres, migration tooling, the default network, database storage, shared server
settings, and the Go port mapping. The override replaces the application image
with Go/Air, clears the demo build and entrypoint, adds source and cache mounts,
and starts Vite. Use both files in this order; the override is not standalone.
Docker Compose must support the `!reset` tag used to remove the demo build and
health check. Project settings in `.zed/settings.json` register this custom tag
with Zed's YAML language server.

The demo health check is disabled in development because Vite does not need
compiled frontend assets. Container status alone does not confirm that Air has
successfully built and started Go; inspect `make logs` for build/startup errors.
The `/api/readyz` endpoint still checks the database and compiled frontend when
requested directly, so it can return 503 until `make build` has run.

## Tests and checks

Run checks from the repository root using the development tool containers. The
compiled demo image does not contain Go or Bun. If the demo is running, stop it
with `docker compose down`, then start development with `make migrate` and
`make up`. Neither command removes stored data.

With development running, use another terminal:

```sh
make check      # Go package tests, Biome lint/format checks, and TypeScript
make test       # Go package tests only; database integration is skipped
make lint       # Frontend lint rules only
make typecheck  # Generate route source and check TypeScript
```

`make check` does not apply Biome fixes. TypeScript checking runs the route
generator, which can update `frontend/src/routetree.gen.ts`; review generated
changes alongside route edits. Biome understands Tailwind directives and excludes
dependencies, build output, and the Bun lockfile. To intentionally apply edits:

```sh
make format     # Format frontend files
make fix        # Apply safe Biome fixes, including import ordering
```

For Go race detection and database integration checks, use a temporary tool
container. This works with the application and frontend stopped:

```sh
make migrate
docker compose -f compose.yaml -f compose.dev.yaml run --rm --no-deps -T \
  -e 'TEST_DATABASE_URL=postgres://gateway:gateway-local-only@postgres:5432/gateway?sslmode=disable' \
  app go test -race ./...
```

The integration tests create and remove uniquely named disposable databases;
the test role needs database-creation privileges. They apply the embedded schema
and check account/credential constraints, session-store compatibility, and
rollback/reapply. The URL above targets the local Compose Postgres service. Without `TEST_DATABASE_URL`, these tests are skipped.

`make build` additionally checks the frontend production build, using a temporary
container and the shared build volume as described above. Package and frontend
checks do not replace a browser walkthrough or verification of live reload,
proxying, and persistence.

## Destructive volume reset

**This deletes the project's stored database data and migration history, plus
its container dependency caches and frontend build output.** Use it only when
intentionally starting over; ordinary shutdown and mode switching use
`make down` or `docker compose down` without `--volumes`.

From the repository root, the combined configuration covers volumes from both
modes, so this command works after either demo or development use:

```sh
docker compose -f compose.yaml -f compose.dev.yaml down --volumes
```

The reset removes `postgres-data`, `go-mod`, `go-build`, `go-tmp`,
`frontend-deps`, `frontend-build`, and `bun-cache` for this Compose project.
It preserves repository files, `.env`, host editor dependencies, and Docker
images. If you used a custom Compose project name, use that same name for the
reset. An external database configured through `DATABASE_URL` is not erased by
this command.

After resetting, choose one mode and repeat its migration-first startup:

- Development: `make migrate`, then `make up`. Container dependencies are
  downloaded again; run `make build` only if you need Go-served compiled assets.
- Demo: follow [Demo with Docker](#demo-with-docker), starting with Postgres and
  the explicit dbmate migration command.

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
| `BROWSER_ORIGIN` | `http://localhost:8080` | Exact browser origin; HTTP requires `localhost`, otherwise use an HTTPS domain. No IP address, trailing slash, path, query, or credentials. |
| `SESSION_LIFETIME` | `12h` | Absolute login-session lifetime, at least 1 second; no idle timeout. |
| `CHALLENGE_LIFETIME` | `5m` | Registration/login challenge lifetime, at least 1 millisecond, enforced server-side. |
| `SHUTDOWN_TIMEOUT` | `5s` | Positive Go duration for draining HTTP requests on SIGINT or SIGTERM. |

Development Compose defaults `BROWSER_ORIGIN` to `http://localhost:5173` for Vite;
demo Compose defaults to port 8080. Leave the variable unset to keep these
mode-specific defaults. Use the 8080 origin when testing Go-served assets in
development. The WebAuthn relying-party ID is derived from the origin hostname
(`localhost` in both modes), without scheme or port. Only the configured origin
is allowed by the WebAuthn configuration; switching ports changes that origin.
Existing `.env` files using `127.0.0.1` must be updated.

Auth configuration uses `go-webauthn/webauthn` and `scs/v2` with `pgxstore`.
Passkeys require discoverable credentials and user verification, with no
attestation requested. Session cookies are named `ssh_term_session`, host-only,
HTTP-only, `SameSite=Strict`, and scoped to `/`. They persist for the session lifetime
and use `Secure` when the configured browser origin uses HTTPS.
Account, credential, and SCS session tables are defined in the account-access
migration; run `make migrate` to apply it. Registration routes now use SCS and
browser-bound, single-use pending state:

- `POST /api/auth/register/begin` accepts `{"display_name":"Alice"}` and returns
  WebAuthn creation options under `publicKey`. It sets an anonymous session cookie.
- `POST /api/auth/register/finish` accepts the browser's serialized WebAuthn
  credential response with that cookie. It consumes the pending attempt before
  verification, even if verification or persistence fails.

Both routes require `Content-Type: application/json` and an `Origin` header
matching `BROWSER_ORIGIN`. Names are trimmed and must contain 1–64 Unicode code
points without control characters. Begin bodies are limited to 4 KiB and finish
bodies to 64 KiB. Each browser session has one pending attempt; a new begin replaces
it. Pending attempts expire after `CHALLENGE_LIFETIME`, and Go/Air restarts clear
them. The process accepts at most 1,024 pending attempts, pruning expired entries
on begin and returning 503 when full. Failed or expired attempts require a new
begin. Missing or invalid pending state and failed verification return 400;
wrong/missing origins return 403 and unsupported content types return 415.

After verification, finish saves the account and credential in one transaction,
then replaces the anonymous session with a fresh authenticated session containing
the account UUID. Success returns HTTP 201 with
`{"account":{"id":"<uuid>","display_name":"Alice"}}` and a new session cookie.
The old session is deleted, its anonymous state is cleared, and the new session
gets the full configured lifetime. Display names may repeat.

An already authenticated browser or duplicate account/credential returns 409.
Database or session-store failures return 503 without a new authenticated cookie.
If account creation committed but the session could not be established, the
account remains saved; sign in with its passkey through the login endpoints. A lost
commit acknowledgement can also leave the account saved. Failed finish attempts
consume the challenge and must not be replayed.

Passkey login is available through two additional JSON POST routes:

- `/api/auth/login/begin` accepts `{}` and returns discoverable assertion options
  under `publicKey`; no display name or account identifier is requested.
- `/api/auth/login/finish` accepts the browser's serialized assertion response
  with the initiating session cookie. Success returns HTTP 200 with the same
  account object as registration and a fresh authenticated session cookie.

Login uses the same exact-origin checks, body limits, challenge lifetime, and
browser binding as registration. Its separate pending store allows one attempt
per browser session and at most 1,024 attempts per process. Begin replaces the
previous login attempt; expiry, restart, and a finish attempt invalidate it.
Malformed or missing/expired state returns 400; unknown credentials, mismatched
user handles, and failed verification return the same generic 401. Already
signed-in sessions return 409. Storage failures return 503 without a new
authenticated cookie. Begin again after a failed finish.

Credential lookup is scoped to RP ID, credential ID, and user handle. A row lock
serializes verification and counter/flag updates, including last-use time.
Zero signature counters are supported. Counter regressions retain the library's
clone warning and previous counter; the warning is advisory and does not by
itself reject a cryptographically valid assertion. Metadata commits before the
new session is saved; if session saving fails, retry with a fresh login ceremony.
Other login sessions for the account remain valid.

Current-user and authorization middleware, the browser passkey UI, and
logout/expiry handling remain pending Slice 2 tasks. The auth endpoints establish
server-side identity but do not yet provide a usable private workspace.

Database URLs are syntax-checked without logging their contents. Startup requires
a reachable database with the expected migration history before opening the HTTP
listener. An explicitly empty `DATABASE_URL` fails; if an older `.env` contains
an empty value, replace it with the development URL in `.env.example`.

In development, changing `HTTP_ADDR` does not change Compose's published port automatically. Keep
its container port mapping and Vite's proxy target in sync, and use an unspecified host (`:8080`) to
accept traffic forwarded into the container.

Normal HTTP shutdown waits up to `SHUTDOWN_TIMEOUT`, then closes remaining HTTP
connections and exits with an error if the deadline expires. Future SSH/WebSocket
sessions need their own cleanup; HTTP shutdown does not close hijacked connections.
Air sends an interrupt and allows 10 seconds before killing Go. Compose's
`APP_STOP_GRACE_PERIOD` defaults to 15 seconds. If increasing the shutdown timeout,
also increase Air's `kill_delay` and Docker's grace period to leave enough time.

Environment changes require recreating the Go service:

```sh
docker compose -f compose.yaml -f compose.dev.yaml up -d --force-recreate app
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
docker compose -f compose.yaml -f compose.dev.yaml run --rm --no-deps -T dbmate new create_accounts
```

Write SQL under the generated `-- migrate:up` and `-- migrate:down` markers. Keep
applied migrations immutable; add a new timestamped migration for subsequent
changes. Apply it with `make migrate`. Air watches SQL files, but if it previously
stopped because a migration was pending, restart Go after applying it:

```sh
docker compose -f compose.yaml -f compose.dev.yaml restart app
```

Write named queries in `db/queries` and run `make generate`. sqlc generates typed
Go methods for pgx in `internal/database/queries`. Review and commit generated Go
with its SQL changes; do not edit it manually. Feature handlers can use these
methods directly without an additional repository layer.
