# Use Air and Vite in development containers

Status: Accepted

## Context

Developers need fast feedback without rebuilding the application image after every edit or changing the lab's routing model.

## Decision

Provide a development Compose override with mounted source. Use Air to rebuild and restart Go and Vite for React and CSS hot updates. Serve the browser through one documented localhost Vite URL and proxy API and terminal WebSocket traffic to Go. Configure auth and origin checks for that browser-facing URL.

Reuse the demo's persistent storage and lab network topology. Keep the normal demo serving compiled assets from Go without development servers.

Run application processes in containers, with matching host Go and Bun toolchains pinned in `.tool-versions` for editor support. Keep host and container dependency installations separate: local Go modules support `gopls`, and local frontend dependencies support TypeScript and editor tooling. Container dependencies remain in Docker volumes. See [the frontend toolchain decision](../frontend/003-bun-and-typescript.md) for Bun and frontend dependency management.

## Consequences

Development has two application processes running in containers. Docker alone can run the application; host editor support requires local toolchains and dependencies. Separate installations consume additional disk space but avoid mixing host and Linux native packages. Keep host and container version pins aligned when upgrading tools.

Air restarts terminate live SSH sessions, and frontend hot updates do not promise terminal-state preservation. Proxy behavior, file watching, and passkey flows need verification in development mode.
