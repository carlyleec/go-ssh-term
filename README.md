# Browser SSH Gateway

A Go and React browser SSH gateway in development. The frontend has a landing
page at `/` and placeholder pages at `/login` and `/connections`. Account access
and SSH features are not implemented; the connections preview is currently public.

## Development with Docker

With Docker running and Make installed, start the Go and frontend development servers:

```sh
make up
```

Open http://127.0.0.1:5173 for the React frontend. The Compose port is bound to
IPv4 loopback; using this address avoids reaching a different service if
`localhost` resolves to IPv6 (`::1`). Vite updates the browser when
you edit React components or styles. The Go server serves the compiled frontend
at http://127.0.0.1:8080 after `make build`. API proxying through Vite remains
a later task.

Air rebuilds and restarts Go when Go source changes. Go build errors appear in
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
make logs       # Follow both services' logs
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
with build instructions. A multi-stage demo image remains a later task.


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

This standalone development setup covers the Go and frontend scaffolds.
Postgres, the demo image, and integration into a shared Compose base with a
development override remain later Slice 1 tasks.

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

Copy `.env.example` to `.env` to customize Docker development settings. `.env` is
ignored by Git; do not commit credentials. Compose passes the values into the Go
container. The Go executable itself reads process environment variables and does
not load `.env` files. Export the variables explicitly when running Go locally.

| Setting | Go default | Meaning |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Listen address in `host:port` form, including `:port` or `[IPv6]:port`. |
| `DATABASE_URL` | Empty | Optional Postgres URL for the upcoming database connection. |
| `BROWSER_ORIGIN` | `http://127.0.0.1:8080` | Exact browser origin, without a trailing slash, path, query, or credentials. |
| `SHUTDOWN_TIMEOUT` | `5s` | Positive Go duration for draining HTTP requests on SIGINT or SIGTERM. |

Compose defaults `BROWSER_ORIGIN` to `http://127.0.0.1:5173` for Vite. Use the
8080 origin when testing Go-served assets. Origin validation here checks config
syntax only; request-origin enforcement and WebAuthn are implemented with auth.
Database URLs are syntax-checked without logging their contents. No database
connection is opened yet, and an empty URL remains allowed until that is added.

Changing `HTTP_ADDR` does not change Compose's published port automatically. Keep
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
