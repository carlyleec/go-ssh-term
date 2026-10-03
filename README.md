# Browser SSH Gateway

A Go and React browser SSH gateway in development. Currently, the Go server
runs with no routes registered. The landing page will be implemented in React.

## Go development with Docker

With Docker running, start the development server:

```sh
docker compose -f compose.dev.yaml up
```

The server listens at http://localhost:8080 and currently returns 404 for all
paths. Air rebuilds and restarts Go when Go source changes. Build errors appear
in the container logs and stop the previous server until the build succeeds.
The first start downloads the Go image and dependencies and compiles Air.

Source is mounted from this checkout. Dependencies, build caches, and generated
binaries live in Docker volumes. The container uses Go 1.26.0 independently of
the host's Go installation. Air is pinned in `go.mod`.

View logs, check Go packages, and stop the development service:

```sh
docker compose -f compose.dev.yaml logs -f app
docker compose -f compose.dev.yaml exec app go test ./...
docker compose -f compose.dev.yaml down
```

Stopping this way preserves the volumes. This Go-only setup is part of S1.1.
React/Vite, Postgres, the demo image, and the full
development Compose override remain later Slice 1 tasks.
