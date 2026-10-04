# Architecture decisions

Short records of accepted architectural choices. Accepted means chosen for implementation, not necessarily implemented. Product behavior and acceptance criteria live in [the PRD](../PRD.md); implementation tasks live in [the development plan](../DEV_PLAN.md).

Number records independently within each area. Each record states its context, decision, and consequences. If a choice changes, add a replacement record and mark the previous one superseded with a link.

## API

- [001 Organize code by vertical slice](api/001-vertical-slices.md)
- [002 Use Postgres for persistent application data](api/002-postgres.md) (superseded by 008)
- [003 Stream terminals over WebSockets](api/003-websocket-terminal-transport.md)
- [004 Separate saved connections from live sessions](api/004-connection-lifecycle.md)
- [005 Support one SSH jump and a limited config importer](api/005-ssh-configuration.md)
- [006 Record connection events without terminal contents](api/006-audit-events.md)
- [007 Use sqlc, pgx, and dbmate for database access](api/007-database-tooling.md) (superseded by 008)
- [008 Use SQLite for local persistent storage](api/008-sqlite-storage.md)
- [009 Separate pool cancellation from SQLite lock waits](api/009-sqlite-lock-wait-deadlines.md)
- [010 Expose owner-scoped SSH key management with bounded multipart uploads](api/010-ssh-key-management-api.md)
- [011 Use Huma typed handlers and generate frontend API contracts](api/011-huma-and-generated-contracts.md)
- [012 Store connection configurations, endpoint trust, and audit snapshots separately](api/012-connection-storage.md)
- [013 Expose owner-scoped saved connection operations](api/013-saved-connection-api.md)
- [014 Bound and authenticate the terminal WebSocket transport](api/014-terminal-websocket-protocol.md)
- [015 Attach one direct SSH shell to each terminal socket](api/015-direct-terminal-shell.md)
- [016 Bind live terminal handles to verified login sessions](api/016-login-owned-terminal-registry.md)
- [017 Cancel terminal work on invalidation and bound stalled I/O](api/017-terminal-lifetime-and-io-bounds.md)
- [018 Persist an audit trail for each terminal attempt](api/018-terminal-audit-events.md)

## Auth

- [001 Authenticate with passkeys and server-side sessions](auth/001-passkeys-and-sessions.md)
- [002 Encrypt uploaded SSH keys separately from login credentials](auth/002-ssh-key-storage.md)
- [003 Require explicit SSH host trust](auth/003-host-verification.md)
- [004 Configure go-webauthn and SCS](auth/004-auth-libraries.md)
- [005 Store account and passkey identities separately](auth/005-account-storage.md)
- [006 Bind registration challenges to browser sessions](auth/006-registration-ceremonies.md)
- [007 Commit registration before issuing an authenticated session](auth/007-registration-session-boundary.md)
- [008 Verify login and update credential metadata together](auth/008-passkey-login.md)

- [009 Verify server sessions before granting workspace access](auth/009-current-user-and-route-protection.md)

- [010 Invalidate the current login session and observe expiry](auth/010-logout-and-session-expiry.md)

- [011 Preserve account-access guarantees on SQLite](auth/011-sqlite-account-persistence.md) (replaces Postgres-specific portions of 004, 005, 007, and 008)

- [012 Persist application encryption material separately and verify it at startup](auth/012-application-encryption-material.md)
- [013 Verify host identity before SSH user authentication](auth/013-ssh-dialing-and-trust-decisions.md)

## Frontend

- [001 Serve a React application from Go](frontend/001-react-and-go.md)
- [002 Use xterm.js in a tabbed workspace](frontend/002-terminal-workspace.md)
- [003 Use Bun and TypeScript for the frontend toolchain](frontend/003-bun-and-typescript.md) (routing choice superseded by 004)
- [004 Use file-based TanStack Router routes](frontend/004-file-based-routing.md)

- [005 Use TanStack Query and Form for account access and workspace data](frontend/005-forms-and-server-state.md)

- [006 Test session UI with real routes and mocked HTTP](frontend/006-rendered-session-tests.md)

- [007 Organize the frontend by routes, shared components, and API modules](frontend/007-frontend-organization.md)
- [008 Attach a disposable xterm.js terminal to the workspace](frontend/008-first-browser-terminal.md)

## Docker

- [001 Provide an isolated OpenSSH lab with Compose](docker/001-compose-lab.md)
- [002 Use Air and Vite in development containers](docker/002-development-workflow.md)

## Open choices

The SQLite driver/session-adapter pairing is recorded in API 008–009 and Auth 011. Accepted SSH key formats, the upload-size limit, and the parsing library are recorded in Auth 002. The SSH library, setup deadline, and host-trust reset interface are recorded in Auth 013. The WebSocket library and protocol limits are recorded in API 014. SSH config syntax and remaining timeout and resource values remain implementation decisions. Coordination between multiple browser tabs is outside v1.
