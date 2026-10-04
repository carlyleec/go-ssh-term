# Development guide

Start with the [development quickstart](../README.md#development-with-docker).
Run commands below from the repository root unless stated otherwise.

## Tools and dependencies

Containers use the versions pinned in [.tool-versions](../.tool-versions).
Docker is sufficient to run the app. For local editor support, install asdf and
its `golang` and `bun` plugins, then run:

```sh
make setup
```

This installs host runtimes, Go modules, and both frontend dependency packages.
Install `gopls` through your editor, use the asdf shims on PATH, and avoid
setting `GOROOT` yourself.

Host `frontend/node_modules` and the container's dependency volume are separate
so native packages match their operating systems. Both use the committed Bun
lockfile. Rerun `make setup` after dependency changes to update host tooling.

With development running, add dependencies using exact versions:

```sh
docker compose -f compose.yaml -f compose.dev.yaml exec frontend bun add --exact PACKAGE
docker compose -f compose.yaml -f compose.dev.yaml exec frontend bun add --dev --exact PACKAGE
```

Review the manifest and lockfile together. After pulling dependency changes,
run `make frontend-restart` to reinstall container dependencies.

## Development services

`make` targets use `compose.yaml` followed by `compose.dev.yaml`. The override
is not standalone; Compose must support its `!reset` tags.

- Vite serves port 5173 and proxies HTTP/WebSocket `/api` traffic to Go.
- Air watches Go and migration files. Frontend edits do not restart Go.
- The contracts service regenerates API artifacts after source changes.
- `make logs` follows app, frontend, and contract logs; `make ps` shows services.

Watchers poll mounted files for Docker Desktop compatibility. A running
container does not guarantee Air compiled successfully; inspect its logs.
Environment changes require recreating the app:

```sh
docker compose -f compose.yaml -f compose.dev.yaml up -d --force-recreate app
```

## Checks

With development services running:

```sh
make check          # Go/frontend tests, Biome check, and TypeScript
make test           # Go tests, including SQLite integration
make frontend-test  # Frontend unit, rendered UI, and contract tests
make lint
make typecheck
make format         # Apply frontend formatting
make fix            # Apply safe Biome fixes
```

For race detection, use a temporary tool container; the app may be stopped:

```sh
docker compose -f compose.yaml -f compose.dev.yaml run --rm --no-deps -T storage-init
docker compose -f compose.yaml -f compose.dev.yaml run --rm --no-deps -T app go test -race ./...
```

SQLite integration tests use temporary files and do not modify the application
database. Run as the non-root container user to include permission checks.

With host dependencies installed, run `bun run test` from `frontend`. Keep the
unit, UI, and contract suites in their separate scripted processes; unit mocks
must not leak into UI tests. Rendered tests use mocked HTTP and terminal objects.
They do not replace browser passkey or live SSH verification.

With the development app and all lab hosts running, check import and real SSH
shells against disposable test storage:

```sh
docker compose -f compose.yaml -f compose.dev.yaml exec -T -e SSH_IMPORT_LAB=1 app \
  go test -race ./internal/connections -run '^TestImportOpenSSHLab$' -count=1 -v
```

This imports the sample config and reads each host's identifying file. It does
not alter saved application connections or trust records. Normal Go tests skip it.

## Frontend builds and routes

`make build` installs locked dependencies, type-checks, and builds Vite assets in
a temporary container. It works without running development services. Go mounts
the build volume read-only; rebuild and refresh to check compiled assets on port
8080. Set `BROWSER_ORIGIN` to that browser origin when testing passkeys there.
Use port 5173 for normal hot-reload development.

Page components live in file-based routes under `frontend/src/routes`.
Vite, type checking, and builds regenerate `frontend/src/routetree.gen.ts`.
Commit route-tree changes alongside route edits; do not edit generated code.
To regenerate manually, run `bun run routes` from `frontend`.

## API contracts and SQL generation

With host tooling installed, run from `frontend`:

```sh
bun run contract:gen    # Go registrations → OpenAPI → TypeScript and Zod
bun run api:gen         # Frontend artifacts from saved OpenAPI
bun run contract:watch # Watch source changes
bun run test:contract
```

Generation does not start HTTP, open SQLite, or read encryption material.
Review `openapi/api.json` and `frontend/src/api/generated/*.gen.ts` alongside
source changes. `make openapi` exports only the OpenAPI schema.

The generator rejects unsupported schema constraints. Extend its tests and
translator when adding a schema feature; form validation schemas remain
handwritten. See [API contract tooling](../adr/api/011-huma-and-generated-contracts.md).

Generator dependencies have their own lockfile in `frontend/scripts` because
the generator and application use different TypeScript compiler versions.
After changing generator dependencies, recreate the watcher:

```sh
docker compose -f compose.yaml -f compose.dev.yaml up -d --force-recreate contracts
```

SQL migrations live in `db/migrations`, and named queries in `db/queries`.
Run `make generate` after query changes and review the generated Go code in
`internal/database/sqlite/queries`. Keep applied migrations immutable; add new
migration files for later changes. Run `make migrate` before starting the app
after schema changes, and `make migrate-status` to inspect the ledger.

## Running Go on the host

Use the pinned dbmate and sqlc versions from Compose when working outside
containers. Build the frontend locally first (`bun run build` in `frontend`).
Then, from the repository root:

```sh
mkdir -p "$PWD/local-data" "$PWD/local-key-material"
chmod 700 "$PWD/local-key-material"
DATABASE_URL="sqlite:$PWD/local-data/gateway.db" dbmate --no-dump-schema up
DATABASE_PATH="$PWD/local-data/gateway.db" ENCRYPTION_KEY_PATH="$PWD/local-key-material/application.key" go run ./cmd/server
```

These local storage directories are ignored by Git. Keep custom database and
key paths outside source control, use separate storage for the encryption key,
and never point two running app processes at the same database file.
