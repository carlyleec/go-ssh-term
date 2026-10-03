# Architecture decisions

Short records of accepted architectural choices. Accepted means chosen for implementation, not necessarily implemented. Product behavior and acceptance criteria live in [the PRD](../PRD.md); implementation tasks live in [the development plan](../DEV_PLAN.md).

Number records independently within each area. Each record states its context, decision, and consequences. If a choice changes, add a replacement record and mark the previous one superseded with a link.

## API

- [001 Organize code by vertical slice](api/001-vertical-slices.md)
- [002 Use Postgres for persistent application data](api/002-postgres.md)
- [003 Stream terminals over WebSockets](api/003-websocket-terminal-transport.md)
- [004 Separate saved connections from live sessions](api/004-connection-lifecycle.md)
- [005 Support one SSH jump and a limited config importer](api/005-ssh-configuration.md)
- [006 Record connection events without terminal contents](api/006-audit-events.md)
- [007 Use sqlc, pgx, and dbmate for database access](api/007-database-tooling.md)

## Auth

- [001 Authenticate with passkeys and server-side sessions](auth/001-passkeys-and-sessions.md)
- [002 Encrypt uploaded SSH keys separately from login credentials](auth/002-ssh-key-storage.md)
- [003 Require explicit SSH host trust](auth/003-host-verification.md)
- [004 Configure go-webauthn and SCS](auth/004-auth-libraries.md)

## Frontend

- [001 Serve a React application from Go](frontend/001-react-and-go.md)
- [002 Use xterm.js in a tabbed workspace](frontend/002-terminal-workspace.md)
- [003 Use Bun and TypeScript for the frontend toolchain](frontend/003-bun-and-typescript.md) (routing choice superseded by 004)
- [004 Use file-based TanStack Router routes](frontend/004-file-based-routing.md)

## Docker

- [001 Provide an isolated OpenSSH lab with Compose](docker/001-compose-lab.md)
- [002 Use Air and Vite in development containers](docker/002-development-workflow.md)

## Open choices

Remaining SSH and WebSocket libraries, accepted SSH key formats and config syntax, timeout and resource values, and the host-trust reset interface remain implementation decisions. Coordination between multiple browser tabs is outside v1. These are not accepted architecture decisions yet.
