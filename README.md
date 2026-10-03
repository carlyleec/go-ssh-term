# Browser SSH Gateway

A Go and React browser SSH gateway in development. The frontend currently shows
an initial placeholder. The landing page and application routes are still to come.

## Development with Docker

With Docker running and Make installed, start the Go and frontend development servers:

```sh
make up
```

Open http://127.0.0.1:5173 for the React frontend. The Compose port is bound to
IPv4 loopback; using this address avoids reaching a different service if
`localhost` resolves to IPv6 (`::1`). Vite updates the browser when
you edit React components or styles. The Go server listens at
http://localhost:8080 and currently returns 404 for all paths. API proxying and
Go serving the compiled frontend will be added in later tasks.

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
