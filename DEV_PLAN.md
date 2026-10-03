# Browser SSH Gateway Development Plan

Implement the slices in order, using [PRD.md](PRD.md) for scope and acceptance criteria. Each slice should produce usable behavior across the frontend, API, and persistence before moving on. Keep feature code together and share only the infrastructure that multiple slices actually need.

## Slice 1 Landing page and runnable application

**Deliverable:** A runnable landing page, persistent Postgres, and a working edit and reload workflow.

**Related ADRs:** [Vertical slices](adr/api/001-vertical-slices.md), [Postgres](adr/api/002-postgres.md), [React and Go](adr/frontend/001-react-and-go.md), [Compose lab](adr/docker/001-compose-lab.md), [Development workflow](adr/docker/002-development-workflow.md).

- [x] **S1.1** Establish `cmd/server` as the Go HTTP entry point and remove redundant starter entry points. Add standalone Go development in `compose.dev.yaml` with pinned Go and Air, mounted source, Docker volumes for dependencies and build output, and watcher exclusions. Document startup, logs, package checks, and shutdown. Verify server startup, Go rebuilds, and compile-error recovery.
  - The server currently has no registered routes and returns 404. The temporary Go landing package was removed; the landing page belongs in React (S1.3), with Go serving compiled assets in S1.4. Future Go features belong under `internal`.
- [x] **S1.2** Create the React TypeScript application with Vite, TanStack Router, Tailwind CSS, and daisyUI. Pin Bun and frontend dependencies and include `bun.lock` in source control. Add a standalone Bun/Vite development service with mounted source, separate host and container dependencies for editor support and platform compatibility, container-managed build output, and hot updates; exclude frontend files from Air. Add pinned Biome formatting, linting, and check scripts with Tailwind support and explicit generated-file exclusions. Pin matching host Go/Bun versions in `.tool-versions`. Add a Makefile for local editor setup, development lifecycle commands, Go tests, frontend checks, formatting, and builds. Document startup, builds, checks, and dependency updates.
  - Frozen-lockfile installation, Biome lint and formatting checks, type checking, production build, HTTP module serving, and Vite WebSocket hot updates pass. The user confirmed the frontend works in the browser. The frontend currently has only a root placeholder; the product pages remain S1.3.
- [ ] **S1.3** Define `/`, `/login`, and `/connections` routes, using placeholders for the later slices. Build the landing page in React.
- [ ] **S1.4** Serve compiled frontend assets from Go. Support direct SPA navigation while keeping missing API endpoints and missing assets from falling through to HTML.
- [ ] **S1.5** Add environment configuration for the HTTP address, database connection, browser-facing origin, and shutdown timeouts. Provide a documented example without secrets.
- [ ] **S1.6** Add a Postgres connection and a minimal SQL migration workflow. Document how to apply migrations and fail startup clearly when required configuration or database setup is missing.
- [ ] **S1.7** Add a multi-stage application Dockerfile and Compose services for the app and Postgres, with persistent database storage and readiness checks.
- [ ] **S1.8** Integrate the Go/Air and Bun/Vite development services from S1.1 and S1.2 with the demo Compose services from S1.7 as a development override. Preserve the existing source mounts, separate host and container frontend dependencies, cache and build volumes, watcher exclusions, Go rebuilds, and frontend hot updates. Reuse the demo's persistent storage and network topology.
- [ ] **S1.9** Configure the Vite development proxy for API requests and terminal WebSockets. Use a single documented browser-facing localhost URL and preserve origin validation.
- [ ] **S1.10** Document demo and development startup, logs, migrations, tests, shutdown, and a separately labeled destructive volume reset.
- [ ] **S1.11** Verify a fresh Compose build, direct navigation, missing API routes, database persistence, Go rebuilds, frontend hot updates, and visible compile errors.

## Slice 2 Account access

**Deliverable:** Passkey registration and login with a protected, user-specific workspace.

**Related ADRs:** [Passkeys and sessions](adr/auth/001-passkeys-and-sessions.md).

- [ ] **S2.1** Select Go WebAuthn and session libraries. Configure the relying-party ID, allowed browser origin, cookie settings, session lifetime, and challenge lifetime for demo and development modes.
- [ ] **S2.2** Add migrations for accounts, one credential per account, and server-side login sessions. Use stable account IDs; display names are not unique authentication identifiers.
- [ ] **S2.3** Implement registration begin and finish endpoints with display-name validation, browser-bound pending registration state, and expiring single-use challenges.
- [ ] **S2.4** Persist the account and verified public credential together only after successful registration. Establish a fresh authenticated session.
- [ ] **S2.5** Implement passkey login begin and finish endpoints, credential lookup, verification, credential metadata updates, and session renewal on login.
- [ ] **S2.6** Build the login page with account creation, display-name entry, passkey prompts, loading states, cancellation handling, and useful error messages.
- [ ] **S2.7** Add a current-user endpoint, authentication middleware, frontend route protection, and origin protection for state-changing requests. Reuse these checks for WebSocket upgrades in Slice 4.
- [ ] **S2.8** Implement logout and session expiration. Establish the session invalidation boundary that terminal cleanup will use in Slice 4.
- [ ] **S2.9** Test expired and reused challenges, wrong origins, failed assertions, unauthenticated access, session renewal, and logout invalidation.
- [ ] **S2.10** Verify registration, login, and logout in the browser through both Go-served assets and the Vite proxy. Document that account recovery is unavailable.

## Slice 3 SSH key management

**Deliverable:** Named, encrypted, persistent SSH keys owned by individual users.

**Related ADRs:** [SSH key storage](adr/auth/002-ssh-key-storage.md).

- [ ] **S3.1** Choose and document accepted private-key formats and an upload-size limit. Validate keys with the SSH library and reject encrypted keys with a clear unsupported-passphrase message.
- [ ] **S3.2** Add the SSH key schema with owner, name, public fingerprint, encrypted private material, and creation time.
- [ ] **S3.3** Generate application encryption material on first initialization and persist it in a separate Docker volume in both run modes. Fail clearly if existing encrypted records cannot be decrypted.
- [ ] **S3.4** Implement authenticated encryption using a standard Go primitive, unique nonces, and explicit error handling. Keep encryption material and plaintext keys out of logs and API responses.
- [ ] **S3.5** Implement user-scoped upload, list, and delete endpoints. Return only safe metadata after upload; do not provide private-key download.
- [ ] **S3.6** Build a key-management modal with upload, naming, list, delete, pending states, and validation errors.
- [ ] **S3.7** Create and label a repository demo-only key pair for the lab. Document upload steps and the consequences of losing either persistent volume.
- [ ] **S3.8** Test malformed and passphrase-protected uploads, encryption round trips, tamper rejection, missing encryption material, and cross-user list/delete attempts.
- [ ] **S3.9** Verify key metadata and decryptability survive container recreation. Add reference-protected deletion when saved connections are introduced in Slice 4.

## Slice 4 Saved connections and a first terminal

**Deliverable:** A saved direct connection opens a usable browser terminal on the bastion.

**Related ADRs:** [WebSocket transport](adr/api/003-websocket-terminal-transport.md), [Connection lifecycle](adr/api/004-connection-lifecycle.md), [Host verification](adr/auth/003-host-verification.md), [Audit events](adr/api/006-audit-events.md), [Terminal workspace](adr/frontend/002-terminal-workspace.md).

- [ ] **S4.1** Add the Ubuntu OpenSSH bastion service on the gateway-facing lab network. Authorize the demo public key, provide a normal shell user, and create a readable `/host-info.txt` identifying the bastion.
- [ ] **S4.2** Persist bastion host keys across ordinary container recreation so trusted fingerprints remain stable. Keep lab SSH ports off the host unless explicitly needed for debugging.
- [ ] **S4.3** Add schemas for saved connections, per-user host trust, and connection audit events. Keep audit destination details meaningful after a saved connection is deleted.
- [ ] **S4.4** Implement user-scoped connection create, list, edit, and delete endpoints with host, port, username, name, and owned-key validation. Prevent deleting keys referenced by saved connections.
- [ ] **S4.5** Build Add connection and Edit connection modals, deletion controls, the Connect picker, and the empty-workspace demo instructions.
- [ ] **S4.6** Implement SSH dialing with bounded timeouts and strict host verification. Add fingerprint approval and rejection, changed-key blocking, and a documented explicit trust-reset path.
- [ ] **S4.7** Define a small WebSocket protocol for terminal data, resize messages, and connection status. Enforce message-size limits and authenticate and validate origin before upgrade.
- [ ] **S4.8** Implement one live terminal path: allocate a PTY, start the shell, forward input and output, and propagate terminal resize events.
- [ ] **S4.9** Keep live connection handles in a Go registry associated with the owning login session. Recheck ownership for every operation that acts on a live connection.
- [ ] **S4.10** Add xterm.js to the workspace with connecting, connected, failed, and disconnected states. Dispose browser listeners, sockets, and terminal objects when no longer needed.
- [ ] **S4.11** Implement cancellation and bounded I/O cleanup for browser disconnect, terminal close, remote exit, failed setup, session invalidation, and server shutdown. Integrate logout and expiry with the live registry now.
- [ ] **S4.12** Record connection start, end, and failure events without terminal contents or secrets. Offer manual reconnect after remote exit or SSH failure; do not reconnect automatically.
- [ ] **S4.13** Test connection ownership, host approval and changed-key rejection, referenced-key deletion, dial failures, terminal close, and logout cleanup using the lab or controlled test peers.
- [ ] **S4.14** Verify typing, control keys, resizing, `cat /host-info.txt`, editing and deleting configurations without disrupting an active shell, and WebSocket behavior through the development proxy.

## Slice 5 Bastion access to private targets

**Deliverable:** Private hosts are reachable through one selected bastion connection.

**Related ADRs:** [SSH configuration](adr/api/005-ssh-configuration.md), [Host verification](adr/auth/003-host-verification.md), [Connection lifecycle](adr/api/004-connection-lifecycle.md), [Compose lab](adr/docker/001-compose-lab.md).

- [ ] **S5.1** Add two Ubuntu OpenSSH targets on a private lab network. Attach the bastion to both lab networks and keep the gateway off the private network; do not publish private-target ports to the host.
- [ ] **S5.2** Authorize the demo public key on both targets, persist their host keys, and add distinct readable `/host-info.txt` files.
- [ ] **S5.3** Extend saved connections with an optional user-owned jump connection. Reject self references, cycles, multi-hop chains, and edits that would turn existing dependencies into multi-hop chains.
- [ ] **S5.4** Add the Jump through selector to connection forms. Block deleting a bastion configuration while another saved connection references it.
- [ ] **S5.5** Dial the target through SSH forwarding on the bastion. Verify each host independently and use the appropriate configured credentials for each hop without copying private keys to the bastion.
- [ ] **S5.6** Extend connection errors, fingerprint prompts, audit events, and cleanup to cover failures at either hop.
- [ ] **S5.7** Verify the gateway cannot reach the private SSH endpoints directly, but can open shells through the bastion and read each identifying file.
- [ ] **S5.8** Test invalid jump references, cross-user references, changed host keys on either hop, target dial failure, and cleanup without disrupting another connection.

## Slice 6 SSH config import

**Deliverable:** The supplied SSH config can create the saved lab connections through a reviewed import.

**Related ADRs:** [SSH configuration](adr/api/005-ssh-configuration.md), [SSH key storage](adr/auth/002-ssh-key-storage.md).

- [ ] **S6.1** Define the precise supported `Host` syntax and single-hop `ProxyJump` forms. Document unsupported wildcard, inheritance, and other OpenSSH behavior rather than silently approximating it.
- [ ] **S6.2** Write the sample config for the bastion and private targets using `Host`, `HostName`, `User`, `Port`, `IdentityFile`, and a single `ProxyJump`.
- [ ] **S6.3** Implement bounded upload and parsing into preview data. Report unsupported directives and syntax, including `ProxyCommand`, without executing commands or reading referenced filesystem paths.
- [ ] **S6.4** Implement user-scoped name-conflict detection, identity-to-uploaded-key mapping, and jump resolution against selected entries or existing configurations.
- [ ] **S6.5** Build the import modal with host selection, parsed settings, key mapping, jump resolution, conflict messages, and a confirmation action.
- [ ] **S6.6** Revalidate the complete import on confirmation and save it transactionally. Reject stale, unauthorized, or unresolved references without partial writes or silent overwrites.
- [ ] **S6.7** Test the sample config, malformed input, unsupported directives, missing keys, deselected jump dependencies, duplicate names, and cross-user references.
- [ ] **S6.8** Verify cancellation writes nothing and a confirmed import can launch terminals on all three hosts. Add the import walkthrough to the README.

## Slice 7 Multiple terminals and connection lifecycle

**Deliverable:** Concurrent terminal tabs and predictable replacement of shells after browser interruption.

**Related ADRs:** [Connection lifecycle](adr/api/004-connection-lifecycle.md), [WebSocket transport](adr/api/003-websocket-terminal-transport.md), [Passkeys and sessions](adr/auth/001-passkeys-and-sessions.md), [Terminal workspace](adr/frontend/002-terminal-workspace.md).

- [ ] **S7.1** Add terminal tabs with independent xterm.js instances and connection state. Allow several live sessions for one saved configuration and resize the terminal when its tab becomes visible.
- [ ] **S7.2** Keep background terminal tabs connected and receiving output. Ensure input is routed only to the intended live SSH session.
- [ ] **S7.3** Define login-session-owned restoration records for previously active terminals, separate from runtime SSH handles. Preserve distinct terminal entries even when they share a saved configuration; do not add multi-browser-tab coordination.
- [ ] **S7.4** Define and document interruption detection, cleanup deadlines, dial timeouts, and bounded automatic replacement attempts. Distinguish browser transport interruption from SSH-side closure and explicit user actions.
- [ ] **S7.5** Implement refresh/interruption recovery that closes obsolete connections before opening replacement shells and rejects duplicate or stale replacement requests. Revalidate session ownership, current configuration, keys, and host trust.
- [ ] **S7.6** Remove restoration intent on explicit terminal close, remote exit, logout, and session expiry. If a required configuration was deleted or reconnect fails, show a disconnected state and manual retry instead of looping.
- [ ] **S7.7** Show that replacement shells start fresh without prior output, running commands, or working directory. Treat server and Air restarts as terminal loss without promising automatic recovery.
- [ ] **S7.8** Make logout and session expiry cancel in-flight dials and prevent new connections from being registered after invalidation. Clear restoration state and close every owned live connection.
- [ ] **S7.9** Bound output buffering, WebSocket writes, and pending work. Define limits for concurrent sessions and terminal scrollback and document their behavior.
- [ ] **S7.10** Test concurrent same-host sessions, independent closure, slow readers, browser disconnect, failed replacement, and logout racing with initial dial or reconnect. Run relevant Go tests with the race detector.
- [ ] **S7.11** Verify leaving or closing the page releases resources within the documented bound, and shutdown closes direct and bastion sessions without growing goroutine or connection counts over repeated cycles.
- [ ] **S7.12** Complete the README walkthrough from a fresh checkout: register, upload the key, import config, approve fingerprints, open all hosts, switch tabs, refresh, disconnect, and log out. Verify data remains after ordinary container recreation.

## Deferred decisions

- Multi-browser-tab coordination remains outside v1. Login-session ownership and logout guarantees still apply.
- Set key formats and upload limits in Slice 3, host-trust reset behavior in Slice 4, exact SSH config syntax in Slice 6, and reconnection timing and resource limits in Slice 7.
- Keep account recovery, multiple passkeys, encrypted SSH key uploads, multiple jump hops, split panes, terminal recording, file transfer, and an audit-history screen outside this plan.
