# Browser SSH Gateway

A Go and React browser SSH gateway in development. The frontend has a landing
page at `/`, passkey account access at `/login`, and a protected
workspace at `/connections`. SSH key management and the Docker bastion are
implemented, along with saved-connection forms, editing, confirmed deletion,
and a Connect picker. The empty workspace explains the local demo setup.
The picker inspects SSH host fingerprints and asks for approval before trusting
an unfamiliar host. Once verified, **Open terminal** opens an interactive shell
in the workspace. One terminal is supported at a time; close it before opening
another.

Saved configurations support `GET`/`POST /api/connections` and
`PUT`/`DELETE /api/connections/{id}`. Create/edit requests require JSON fields
`name`, `host`, `port`, `username`, and `ssh_key_id`, an authenticated session,
and the configured Origin. Referenced SSH keys cannot be deleted (409 conflict).
See the [API decision](adr/api/013-saved-connection-api.md) for validation and
response details. Apply pending migrations with `make migrate` before running
the updated API against an existing database.

## SSH host verification and trust reset

Choose **Connect**, then a saved destination. Compare the displayed SHA-256
fingerprint with a trusted source before choosing **Approve fingerprint**.
**Reject and close** stores nothing. Inspection and approval stop before user
authentication; approval is remembered for your account and that host/port.
The backend terminal endpoint supports key-based SSH login and a PTY-backed
shell after approval. Choose **Open terminal** after verification to use it in
the browser. Try `cat /host-info.txt` on the local bastion.

A changed host key blocks connection. Verify why the identity changed first
(for example, the lab host-key volume was intentionally replaced). In the
fingerprint dialog, choose **Reset host trust**, review the old fingerprint,
then **Confirm trust reset**. This removes trust for all your saved connections
to that endpoint. Inspect the host again and explicitly approve the replacement;
reset never approves it automatically. Other users' trust is unchanged.

The reset API is `POST /api/connections/{id}/host-trust/reset`, with the current
normalized `host`, `port`, and previously trusted `fingerprint` in a JSON body,
an authenticated cookie, and the configured Origin. Stale decisions return 409.
If the original configuration was deleted, create another for the same endpoint
to inspect/reset its retained trust. See [SSH trust decisions](adr/auth/013-ssh-dialing-and-trust-decisions.md).

## Terminal WebSocket protocol

`GET /api/connections/{id}/terminal` requires an authenticated session cookie,
the exact configured Origin, an owned saved connection, and the WebSocket
subprotocol `ssh-terminal.v1`. Terminal data uses binary messages up to 32 KiB;
resize/status controls use JSON text up to 1 KiB. See the
[protocol decision](adr/api/014-terminal-websocket-protocol.md) for message shapes,
dimension limits, close codes, and write bounds.

After upgrade, the endpoint sends `connecting`, verifies SSH host trust, opens
an `xterm-256color` PTY (initially 80×24), starts a shell, and sends `connected`.
Binary input/output and JSON resize events then reach the shell. Setup failure
sends safe `failed` status; remote exit drains output and sends `disconnected`.
See the [shell decision](adr/api/015-direct-terminal-shell.md) for setup bounds
and cleanup behavior. No automatic reconnect occurs.

Each terminal has a distinct in-memory handle owned by the verified login
session, including when several sockets use the same saved configuration.
Input, resize, and close operations recheck that ownership. The absolute session
deadline cancels pending setup and closes live shells; editing/deleting a saved
configuration does not affect a running shell. See the
[registry decision](adr/api/016-login-owned-terminal-registry.md).

The browser shows connection status and preserves terminal output after the
shell ends. **Reconnect** reloads the current saved configuration and opens host
verification again; **Open terminal** replaces the ended terminal with a fresh
shell. Previous output, commands, and working directory are not restored. A
deleted configuration cannot be reconnected. **Close terminal** releases the
browser resources. Leaving the workspace or confirmed
session cleanup closes its socket. There is no automatic reconnect or shell
restoration. The terminal retains 1,000 scrollback lines and disconnects if its
pending input/output queue exceeds 1 MiB.

Logout cancels pending setup and closes shells belonging to that login before
returning success. Other logins remain connected. Server shutdown closes and
waits for terminal work alongside HTTP requests. Heartbeats detect silent peers,
and five-second I/O deadlines release stalled connections; see the
[lifetime and I/O bounds](adr/api/017-terminal-lifetime-and-io-bounds.md).
Each admitted terminal attempt records a `start` event in SQLite before dialing,
then an optional safe-coded `failure` and an `end` after cleanup. Records retain
the destination used even after configuration edits/deletion; they exclude
terminal contents and secrets. Failed start persistence prevents dialing. Failed
completion persistence leaves incomplete history and a diagnostic containing only
the attempt ID. See [audit behavior](adr/api/018-terminal-audit-events.md).
Multiple terminal tabs remain subsequent work.

Saved connections also accept an optional `jump_connection_id` through the API.
It must identify a direct connection owned by the same account. Self references,
cycles, and multiple jumps are rejected; a referenced bastion cannot be deleted
until its dependents are updated or removed (409 response). Omit the field or send
`null` to save a direct connection. In Add/Edit connection, **Jump through** offers
owned direct destinations, excluding the connection being edited. Choose
**None — connect directly** to remove a jump. Referenced bastions must stay direct;
change or remove their dependent references before deleting them. Saved rows show
the selected jump name. Unavailable choices require an explicit new selection.
SSH forwarding uses the selected bastion, verifying each host independently and
using each connection’s configured credentials. Select **Connect** on the target:
the dialog first requests any needed bastion approval, then independently checks
the target fingerprint. Changed keys require explicit reset and fresh approval.
Only a verified target enables **Open terminal**. Setup errors identify the
bastion or target operation that failed; audit events retain both destinations.
See [saved jump connections](adr/api/019-saved-jump-connections.md).

After updating to migration `20261004000300`, run `make migrate`, then `make up`.
The server refuses to start with a pending migration; the browser otherwise shows
“Could not check your session.” Migration preserves existing direct connections.

## SSH lab bastion

Both Compose modes include an Ubuntu OpenSSH bastion. The gateway reaches it at
`bastion:22` on the `lab-gateway` network. SSH has no published host port. The
normal shell user is `demo`, authenticated with the supplied
[demo key](demo/keys/README.md); root and password logins are disabled.
Running `cat /host-info.txt` in its shell prints `Bastion host: bastion`.

To build and start just the bastion:

```sh
docker compose up --build -d --wait bastion
docker compose logs bastion
```

The lab also includes `target-1` and `target-2`, Ubuntu OpenSSH hosts on the
internal `lab-private` network. Only the bastion joins both `lab-gateway` and
`lab-private`; the application does not join the private network. Neither target
publishes a host port. Both targets use port `22`, user `demo`, and the same
supplied demo key. Their `/host-info.txt` files identify the individual target.

Start all three SSH hosts with:

```sh
docker compose up --build -d --wait bastion target-1 target-2
```

`make up` includes all three hosts. Save a direct connection to `bastion:22`,
then save connections to `target-1:22` and `target-2:22` with that bastion selected
under **Jump through**. Connect to either target and follow the fingerprint
verification prompts. The private targets cannot be reached directly by the application.
Each target keeps its host keys in its own named volume (`target-1-host-keys`
and `target-2-host-keys`), shared by development and demo modes. Ordinary
recreation preserves these identities, just as it does for the bastion.

Host keys are generated on first startup in the `bastion-host-keys` volume,
separate from the image and SSH configuration. Both Compose modes share this
volume when using the same project name. Ordinary shutdown, image rebuilds, and
container recreation preserve the keys and their fingerprints.

To check persistence, run this fingerprint command before and after
`docker compose up -d --force-recreate --wait bastion`; all three fingerprints
should match:

```sh
docker compose exec bastion sh -c 'for key in /var/lib/ssh-host-keys/*_key.pub; do ssh-keygen -lf "$key"; done'
```

Deleting the volume deliberately replaces the bastion's identity on its next
startup. Upgrading from the original container-only keys also changes its
identity once; those old keys are not migrated. Clients that trusted the old
identity will report a changed host key.

## Demo with Docker

Docker builds the React assets and Go executable; no host Go or Bun installation
is needed. From a fresh checkout, initialize SQLite storage and explicitly apply
migrations before starting the application:

```sh
docker compose run --rm --no-deps storage-init
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
Stop the app before applying later migrations (`docker compose stop app`).

```sh
docker compose ps
docker compose logs -f app
docker compose run --rm --no-deps dbmate status
docker compose down
```

Ordinary shutdown preserves the `sqlite-data` directory volume, including the
database and SQLite journal files, the separate `encryption-key` volume, and
the `bastion-host-keys`, `target-1-host-keys`, and `target-2-host-keys` volumes.
Demo and development use the same Compose project and these persistent volumes when run from this directory;
stop one mode before starting the other (`make down` for development,
`docker compose down` for the demo). Do not add `--volumes` when switching modes.

`GET /api/readyz` returns 204 when a database schema read succeeds and the frontend
entry point is readable, or 503 otherwise. The database probe has a two-second
context deadline; an external SQLite lock wait can last up to the separate
five-second busy timeout. Docker probes every five seconds and marks the app unhealthy after
three failures; it does not automatically restart an unhealthy container.
Migration history is checked at startup, not on each readiness request.

## Development with Docker

With Docker running and Make installed, start the Go and frontend development servers:

```sh
make migrate
make up
```

Open http://localhost:5173 for development pages, API requests, and terminal
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
The proxy preserves origin headers for backend validation. Account mutations
and terminal upgrades enforce the configured origin. No permissive CORS setting is needed.
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
make logs       # Follow Go and frontend logs
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
SQLite storage initialization, migration tooling, the default network, shared server
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
make check      # Go and frontend tests, Biome checks, and TypeScript
make test       # Go package tests, including SQLite integration
make frontend-test # Frontend helper and rendered UI tests
make lint       # Frontend lint rules only
make typecheck  # Generate route source and check TypeScript
```

Frontend tests run in separate Bun processes: helper tests in `tests/*.test.ts`
and rendered UI tests in `tests/ui`. The UI suite uses React Testing Library,
Happy DOM, the real route tree, and a fresh QueryClient with mocked HTTP responses.
A controlled Query clock exercises polling and background expiry without real
30-second waits. This checks session UI wiring, not native passkey behavior or
real-browser timer throttling. Those remain browser walkthrough checks.

With pinned host dependencies installed, run `bun run test` from `frontend`, or
use `bun run test:unit` / `bun run test:ui` for either suite. Use these scripts
rather than running every test in a single Bun process: helper module mocks must
not leak into UI tests, and the UI suite needs its DOM preload. `make typecheck`
also checks test files through `frontend/tsconfig.test.json`.

`make check` does not apply Biome fixes. TypeScript checking runs the route
generator, which can update `frontend/src/routetree.gen.ts`; review generated
changes alongside route edits. Biome understands Tailwind directives and excludes
dependencies, build output, and the Bun lockfile. To intentionally apply edits:

```sh
make format     # Format frontend files
make fix        # Apply safe Biome fixes, including import ordering
```

For Go race detection and database integration checks, use a temporary tool
container. This also works with the app and frontend stopped:

```sh
docker compose -f compose.yaml -f compose.dev.yaml run --rm --no-deps -T storage-init
docker compose -f compose.yaml -f compose.dev.yaml run --rm --no-deps -T app go test -race ./...
```

Integration tests run by default on independent files in `t.TempDir()` with
production migrations and connection settings. No database service or
`TEST_DATABASE_URL` is needed, and tests do not modify the application's database.
They cover schema constraints, complete credential metadata, transaction and
session failures, expiry, concurrent writes, cleanup, reconnects, pool/query
cancellation, and external lock timeout/recovery. Run as the non-root container
user to include file/directory permission checks.

`make build` additionally checks the frontend production build, using a temporary
container and the shared build volume as described above. Package and frontend
checks do not replace a browser walkthrough or verification of live reload,
proxying, and persistence.

## OpenAPI contract

The contract covers all nine auth and SSH-key operations. With pinned host tools
installed (`make setup`), run these commands from `frontend`:

```sh
bun run contract:gen    # Go → OpenAPI → TypeScript and Zod
bun run api:gen         # Generate frontend files from saved OpenAPI only
bun run contract:watch # Regenerate after Go/source changes; Ctrl-C stops it
bun run test:contract  # Check Zod generation and representative payloads
```

Generation uses the runtime Huma registrations without starting HTTP, opening
SQLite, or reading encryption material. Artifacts live in `openapi/api.json` and
`frontend/src/api/generated/{schema,zod}.gen.ts`; review and commit them alongside
source edits. Generated frontend files have do-not-edit banners and are excluded
from Biome. `make openapi` remains available for exporting just the Go spec.

The Zod generator covers request bodies, response envelopes, referenced schemas,
multipart files (`File`), and empty responses (`undefined`). It rejects unknown
schema keywords/formats instead of dropping constraints. Extend the translator
and its tests when adding a new schema feature. Form schemas remain handwritten.
Account and SSH-key queries use generated types and validate responses with the
generated schemas; passkey browser calls retain the browser SDK's input types.

`make up` also starts the `contracts` watcher service; `make logs` includes its
output. It needs neither the app nor database to generate contracts. The watcher
polls source contents for bind-mount compatibility, debounces changes, and runs
one generation at a time. It ignores generated outputs and dependencies. Compile
or generation errors are logged, preserve the last-good artifacts, and are
retried after the next source edit. Both generators finish before outputs are
replaced, and unchanged files are not rewritten.

Generation tools have their own locked package in `frontend/scripts`: the app
uses TypeScript 7, while `openapi-typescript` requires the TypeScript 5 compiler
API. `make setup` installs both packages. After changing generator dependencies,
recreate the Compose watcher:

```sh
docker compose -f compose.yaml -f compose.dev.yaml up -d --force-recreate contracts
```

Its Go caches and dependency volume are separate from the app's. This demo uses
local verification and has no CI.

## Destructive volume reset

**This deletes the project's stored database data, migration history, and application encryption key, plus
its container dependency caches and frontend build output.** Use it only when
intentionally starting over; ordinary shutdown and mode switching use
`make down` or `docker compose down` without `--volumes`.

From the repository root, the combined configuration covers volumes from both
modes, so this command works after either demo or development use:

```sh
docker compose -f compose.yaml -f compose.dev.yaml down --volumes
```

The reset removes `sqlite-data`, `encryption-key`, `bastion-host-keys`,
`target-1-host-keys`, `target-2-host-keys`, `go-mod`, `go-build`, `go-tmp`,
`frontend-deps`, `frontend-build`, and `bun-cache` for this Compose project.
It preserves repository files, `.env`, host editor dependencies, and Docker
images. If you used a custom Compose project name, use that same name for the
reset. The inactive legacy `postgres-data` volume is not declared in this
configuration and is preserved. Do not delete it as part of the SQLite transition.
A host database outside these named volumes is not erased by this command.

After resetting, choose one mode and repeat its migration-first startup:

- Development: `make migrate`, then `make up`. Container dependencies are
  downloaded again; run `make build` only if you need Go-served compiled assets.
- Demo: follow [Demo with Docker](#demo-with-docker), starting with storage initialization and
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
| `DATABASE_PATH` | Required | Absolute, clean path to an existing migrated SQLite file. Compose defaults to `/data/gateway.db` inside the shared directory volume. |
| `ENCRYPTION_KEY_PATH` | Required | Absolute, clean file path in a separate private directory. Compose defaults to `/key-material/application.key` in the `encryption-key` volume. |
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

Auth configuration uses `go-webauthn/webauthn` and `scs/v2` with the local
context-aware SQLite store and the same database pool as application queries.
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

Credential lookup is scoped to RP ID, credential ID, and user handle. An immediate SQLite
transaction reserves the writer before lookup and serializes verification and
counter/flag updates, including last-use time.
Zero signature counters are supported. Counter regressions retain the library's
clone warning and previous counter; the warning is advisory and does not by
itself reject a cryptographically valid assertion. Metadata commits before the
new session is saved; if session saving fails, retry with a fresh login ceremony.
Other login sessions for the account remain valid.

The browser passkey UI, current-user endpoint, and authorization middleware are
implemented. The workspace includes sign-out and observes session expiry.

Database paths must be absolute filesystem paths; relative paths, SQLite URIs,
and in-memory alternatives are rejected. Startup requires an existing, migrated
file and writable file/directory permissions before opening the HTTP listener.
An explicitly empty `DATABASE_PATH` fails. Replace old `DATABASE_URL` settings
with the path in `.env.example`; old Postgres accounts and sessions are not imported.
Register a fresh account with a passkey after migrating to SQLite.

In development, changing `HTTP_ADDR` does not change Compose's published port automatically. Keep
its container port mapping and Vite's proxy target in sync, and use an unspecified host (`:8080`) to
accept traffic forwarded into the container.

Server shutdown stops terminal admission, closes SSH/WebSocket transports, and
waits for terminal workers alongside HTTP draining within `SHUTDOWN_TIMEOUT`.
It closes remaining HTTP connections and exits with an error if that deadline
expires. The terminal registry handles hijacked sockets separately because
HTTP shutdown alone does not close or wait for them.
Air sends an interrupt and allows 10 seconds before killing Go. Compose's
`APP_STOP_GRACE_PERIOD` defaults to 15 seconds. If increasing the shutdown timeout,
also increase Air's `kill_delay` and Docker's grace period to leave enough time.

Environment changes require recreating the Go service:

```sh
docker compose -f compose.yaml -f compose.dev.yaml up -d --force-recreate app
```

## Database and queries

Both modes share `/data/gateway.db` in the named `sqlite-data` directory volume.
Keep the database, `-wal`, and `-shm` files together; do not mount only the database
file. Use a local filesystem and one app process at a time. Stop one mode before
starting the other. Ordinary container recreation, Air restarts, and `make down`
preserve accounts and unexpired sessions. Pending passkey challenges are intentionally
process-local and must be restarted after an app restart.

The app, development Air process, and dbmate run as UID/GID 65532. The
`storage-init` service gives that user ownership of the database directory;
the development override also prepares Go cache volumes. Migrations therefore
create files writable by both serving modes. Custom `DATABASE_PATH` values must
stay in a writable mounted directory shared with dbmate. The application encryption
key uses the separate `encryption-key` volume, which dbmate does not mount.

On first startup, Go generates a random 32-byte application encryption key at
`ENCRYPTION_KEY_PATH` only if no SSH key records exist. The key file has mode 0600;
its parent directory must exist with owner-only permissions (0700). Both Compose
modes initialize that directory for UID/GID 65532 and reuse the same file after
restarts and container recreation. Custom paths must stay in a separate persistent
mount, outside the database volume and source control. Existing installations need
storage initialization and app recreation to pick up the new volume and setting.

Before serving HTTP, startup verifies that every stored SSH key can be decrypted.
Missing, malformed, unreadable, or incorrect application key material fails startup;
it is never silently replaced while SSH key records exist. Restore the matching
key-volume backup and check path/permissions. Losing this volume without a backup
requires deliberately discarding unusable encrypted records and re-uploading the
original SSH keys; automatic recovery or deletion is not implemented. Losing the
SQLite volume loses the accounts, sessions, and uploaded key records, even if the
encryption key survives. Back up both volumes. Access to both defeats the protection
against disclosure from a database copy alone. The [intentionally public demo SSH identity](demo/keys/README.md) is separate
from this secret application encryption key.

Run `make migrate` before the first startup and whenever you pull new migrations.
It stops the app, initializes storage permissions, and runs pinned dbmate with
strict ordering. Restart with `make up` afterward. Repeated migration runs apply
only pending files; `make migrate-status` shows the ledger without stopping the
app. The Go process checks migration history but never applies migrations.
Missing, pending, or unknown versions prevent startup with actionable errors.
This checks history, not manual schema drift.

The shared pool has one open/idle connection. Every connection uses foreign keys,
WAL, `synchronous=FULL`, a five-second busy timeout, and immediate transactions.
Database operations have a ten-second context budget, preserving earlier caller
deadlines. Pool waits are cancelable; external SQLite lock waits may outlast context
cancellation until the busy timeout. Readiness retains its two-second context.
Transactions close before session-store access to avoid waiting for their own
connection. Shutdown stops session cleanup before closing the pool.

The active SQLite lineage is in `db/migrations`, with named queries in `db/queries`.
`make generate` uses pinned sqlc to generate `database/sql` methods in
`internal/database/sqlite/queries`. Review generated Go with its SQL changes;
do not edit generated code manually. Creation/last-use timestamps and session
expiry are UTC Unix nanoseconds. Keep applied migrations immutable and add
new timestamped files for later changes.

Original Postgres migrations remain unchanged in `db/legacy/migrations`, outside
embedding, generation, and dbmate's active path. The old Postgres volume is left
intact; no data import or automatic deletion occurs. Existing accounts must
register again. Old cookies grant no access to the new database.

To run Go on the host, migrate a local file explicitly with dbmate and export its
absolute `DATABASE_PATH`; do not point two running apps at the same file. The
pinned tools are dbmate 2.36.0 and sqlc 1.31.1. For example, with those installed:

```sh
mkdir -p "$PWD/local-data"
mkdir -p "$PWD/local-key-material"
chmod 700 "$PWD/local-key-material"
DATABASE_URL="sqlite:$PWD/local-data/gateway.db" dbmate --no-dump-schema up
DATABASE_PATH="$PWD/local-data/gateway.db" ENCRYPTION_KEY_PATH="$PWD/local-key-material/application.key" go run ./cmd/server
```

Build `frontend/dist` first for host serving. The example's local storage directories
are excluded from Git and Docker build context. Keep any custom database and key
paths outside source control and initialize/migrate while the app is stopped.

### SSH key-management API

To try the key-management modal, follow the [demo key upload steps](demo/keys/README.md#upload).
The supplied key is intentionally public and only for the disposable local lab.

All key endpoints require a signed-in session. Upload and delete also require
the exact configured `Origin`. They do not extend the session lifetime.

| Method and path | Request | Success |
| --- | --- | --- |
| `POST /api/keys` | Multipart `name` and `private_key` fields | 201, `{"key": {...}}` |
| `GET /api/keys` | No body | 200, `{"keys": [...]}` containing only your keys |
| `DELETE /api/keys/{id}` | No body | 204; missing or another user's key returns 404 |

Upload one unencrypted OpenSSH Ed25519 private key, at most 16 KiB. Requests are
limited to 32 KiB including multipart overhead; unknown or duplicate fields are
rejected. The name field accepts at most 256 bytes and is trimmed to 1–64 UTF-8
characters without control characters. Let the browser set the Content-Type
boundary when submitting FormData. Upload parsing streams in memory and does not
spool private keys to temporary files.

Each metadata object contains only `id`, `name`, `public_fingerprint`, and a UTC
`created_at` timestamp. Lists are newest first; duplicate names and fingerprints
are allowed. Uploads are validated and encrypted before being saved. Neither the
plaintext nor ciphertext is returned, and no private-key download route exists.
Errors use `{"error":"..."}`: invalid input is 400, excessive size 413,
unsupported content type 415, and storage/encryption failure 503. Responses are
not cached. Deletion protection for keys used by saved connections will be added
with those connections.

### Verify uploaded-key persistence

Upload the [demo key](demo/keys/README.md#upload) and note its name, fingerprint,
and creation time in **Manage SSH keys**. Recreate the app for the mode you are
running, retaining both volumes:

```sh
# Development
docker compose -f compose.yaml -f compose.dev.yaml up -d --no-deps --force-recreate app

# Demo (use this instead when running demo mode)
docker compose up -d --no-deps --force-recreate --wait app
```

Wait for the server to start, refresh the workspace, and reopen **Manage SSH keys**.
Sign in again if your session has expired. The saved metadata should be unchanged;
there should be no need to upload again. Successful backend startup also means
its decryption check passed for every stored key. Actual SSH authentication is
verified when lab connections are introduced.

Do not remove volumes during this check. When checking a switch between demo and
development, stop the current mode first and use the same Compose project name so
both modes retain the same database and encryption-key volumes. Development serves
assets through Vite; its backend readiness endpoint requires a compiled frontend
and can return 503 when those assets have not been built. Use the workspace/session
API to check development availability.

### Current user and protected workspace

`GET /api/auth/me` returns `{"account":{"id":"…","display_name":"…"}}`
for a valid session and existing account in the configured RP. Anonymous, invalid,
or expired sessions return 401; session/database failures return 503. Identity
responses are not cached by HTTP and do not extend the session lifetime.

The `/connections` route checks this endpoint before rendering and redirects
signed-out users to `/login`. Signed-in users visiting `/login` return to the
workspace. Connection failures show a retry screen; account recovery remains
unavailable. The workspace rechecks the session every 30 seconds while visible
and on visibility/reconnect. A confirmed expired session returns to login;
background timer throttling may delay UI updates, but server checks still reject
expired sessions immediately.

Future private API handlers must use `Access.Require` and scope resource queries
to `AccountFromContext`. The middleware also enforces the configured Origin on
unsafe requests and WebSocket upgrades. Serving the SPA shell does not authorize
API access.

`POST /api/auth/logout` requires the configured Origin and JSON content type. It
deletes only the current server session, expires its cookie, and returns 204.
Missing or already expired sessions also return 204. Storage failures return 503
without reporting successful logout. The workspace's Sign out button clears
cached data after success; uncertain failures offer a manual retry.

Terminals use `SessionFromContext` for their non-secret login-session owner ID
and absolute deadline. After session deletion, `Access.OnLogout` synchronously
revokes registry admission, cancels pending setup, closes owned shells, and waits
for their workers. Requests authenticated before logout cannot publish a shell
after invalidation. Absolute expiry independently cancels terminal work; database
cleanup is not an expiry notification.
