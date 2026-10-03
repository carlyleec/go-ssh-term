# Use Air and Vite in development containers

Status: Accepted

## Context

Developers need fast feedback without rebuilding the application image after every edit or changing the lab's routing model.

## Decision

Provide a development Compose override with mounted source. Use Air to rebuild and restart Go and Vite for React and CSS hot updates. Serve the browser through one documented localhost Vite URL and proxy API and terminal WebSocket traffic to Go. Configure auth and origin checks for that browser-facing URL.

Reuse the demo's persistent storage and lab network topology. Keep the normal demo serving compiled assets from Go without development servers.

## Consequences

Development has two application processes, with tools and caches managed in containers. Air restarts terminate live SSH sessions, and frontend hot updates do not promise terminal-state preservation. Proxy behavior, file watching, and passkey flows need verification in development mode.
