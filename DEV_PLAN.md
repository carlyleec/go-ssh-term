# Browser SSH Gateway Development Plan

Implement the slices in order, using [PRD.md](PRD.md) for scope and acceptance criteria. Each slice should produce usable behavior across the frontend, API, and persistence before moving on. Keep feature code together and share only the infrastructure that multiple slices actually need.

## Slice 1 Landing page and runnable application

**Deliverable:** A runnable landing page, persistent Postgres, and a working edit and reload workflow.

**Related ADRs:** [Vertical slices](adr/api/001-vertical-slices.md), [Postgres](adr/api/002-postgres.md), [Database tooling](adr/api/007-database-tooling.md), [React and Go](adr/frontend/001-react-and-go.md), [Compose lab](adr/docker/001-compose-lab.md), [Development workflow](adr/docker/002-development-workflow.md).

- [x] **S1.1** Establish `cmd/server` as the Go HTTP entry point and remove redundant starter entry points. Add standalone Go development in `compose.dev.yaml` with pinned Go and Air, mounted source, Docker volumes for dependencies and build output, and watcher exclusions. Document startup, logs, package checks, and shutdown. Verify server startup, Go rebuilds, and compile-error recovery.
  - The Go entry point is ready for feature routes. The temporary Go landing package was removed; the landing page belongs in React (S1.3), with Go serving compiled assets in S1.4. Future Go features belong under `internal`.
- [x] **S1.2** Create the React TypeScript application with Vite, TanStack Router, Tailwind CSS, and daisyUI. Pin Bun and frontend dependencies and include `bun.lock` in source control. Add a standalone Bun/Vite development service with mounted source, separate host and container dependencies for editor support and platform compatibility, container-managed build output, and hot updates; exclude frontend files from Air. Add pinned Biome formatting, linting, and check scripts with Tailwind support and explicit generated-file exclusions. Pin matching host Go/Bun versions in `.tool-versions`. Add a Makefile for local editor setup, development lifecycle commands, Go tests, frontend checks, formatting, and builds. Document startup, builds, checks, and dependency updates.
  - Frozen-lockfile installation, Biome lint and formatting checks, type checking, production build, HTTP module serving, and Vite WebSocket hot updates pass. The user confirmed the frontend works in the browser.
- [x] **S1.3** Define `/`, `/login`, and `/connections` using file-based TanStack Router routes, using placeholders for the later slices. Build the landing page in React.
  - Added route generation for Vite and standalone type checks, page components in route files, shared navigation, a skip-to-content link, and a not-found page. Biome, TypeScript, production build, route loading/rendering, and direct Vite requests pass. Go-served navigation is covered by S1.4; the landing page received a visual browser check in S1.7.
- [x] **S1.4** Serve compiled frontend assets from Go. Support direct SPA navigation while keeping missing API endpoints and missing assets from falling through to HTML.
  - Serve `frontend/dist` from disk and share the frontend build volume read-only with Go. Cover SPA navigation, static files, API precedence, missing assets, methods, and missing builds with focused Go tests. Verify HTTP responses against the built frontend; the image-built landing page received a visual browser check in S1.7.
- [x] **S1.5** Add environment configuration for the HTTP address, database connection, browser-facing origin, and shutdown timeouts. Provide a documented example without secrets.
  - Validate configuration at startup and implement bounded HTTP shutdown on SIGINT/SIGTERM. Pass development settings through Compose and allow graceful shutdown through Air. Required database configuration and connectivity are covered by S1.6; request-origin enforcement remains with authentication.
- [x] **S1.6** Add a shared pgx pool, sqlc-generated queries, and an explicit dbmate SQL migration workflow. Require database configuration and check connectivity and migration history before serving HTTP. Add development Postgres with persistent storage and readiness checks, pinned migration/query tools, Make targets, and workflow documentation.
  - Verified Go tests with the race detector and disposable Postgres databases, rejection of unmigrated databases, repeatable migrations, deterministic query generation, migration history surviving container recreation, and successful HTTP startup. The baseline introduces migration history; feature tables belong to later slices.
- [x] **S1.7** Add a multi-stage application Dockerfile and demo Compose setup serving compiled React from Go as a non-root user. Reuse Postgres storage, readiness checks, and explicit dbmate tooling through shared service definitions. Add bounded database/frontend readiness checks and document demo startup, migrations, logs, status, shutdown, and switching modes without removing data.
  - Verified image build, fresh migration-first startup and rejection of unmigrated databases, repeatable migrations, direct page and asset responses, missing API/asset 404s, landing-page browser rendering, readiness failure/recovery during database downtime, and migration persistence after recreation. Container Go race tests pass, including readiness and database integration tests.
- [x] **S1.8** Convert development Compose into an override of the demo base, sharing Postgres, migration tooling, network, storage, and common server settings. Replace the demo build and entrypoint with Go/Air, remove the asset-dependent demo health check, and preserve Vite, source mounts, separate host/container dependencies, cache/build volumes, and watcher exclusions. Update Make targets and documented commands to load both files.
  - Verified merged configuration and runtime mounts, startup with `--build --wait`, startup without compiled frontend assets, Go compile-error visibility and rebuild recovery, CSS/React hot-update delivery, and migration persistence while switching between modes. Go tests, Biome, and TypeScript checks pass. Zed project settings register the Compose `!reset` tag for YAML validation.
- [x] **S1.9** Proxy `/api` and `/api/...` HTTP requests and WebSocket upgrades through Vite to Go, preserving paths, Host, Origin, and cookies. Document port 5173 as the development browser origin and reserve port 8080 for direct debugging and compiled-asset checks.
  - Verified live API readiness and missing-route responses through Vite, page navigation, and Vite's own HMR WebSocket. A temporary backend verified HTTP forwarding, text/binary WebSocket round trips, preserved browser headers/cookies, and backend wrong-origin rejection. Biome and TypeScript checks pass. Application origin enforcement remains in S2.7 and terminal handlers in Slice 4.
- [x] **S1.10** Consolidate package, frontend, race, and database integration test instructions, including prerequisites and generated-route behavior. Document a separate destructive volume reset with its data/cache scope, preserved files, and migration-first recovery steps; lifecycle and browser URL instructions are covered by S1.7–S1.9.
  - Checked documented commands against Make dry runs, Compose CLI help, and the configured volume list. The destructive reset was not executed; the fresh-checkout walkthrough remains S1.11.
- [x] **S1.11** Run a final fresh-checkout walkthrough in both modes after proxy integration, including direct navigation, missing API routes, database persistence, Go rebuilds, frontend hot updates, and compile-error recovery.
  - Verified a clean tracked-file export with fresh isolated volumes: uncached demo image build, explicit migrations, demo navigation/refresh, development page and API routing, readiness before/after the frontend build, React/CSS hot updates, Go compile-error recovery, and database row persistence across recreation in both modes. Go race and database integration tests, Biome, TypeScript, and the frontend production build pass. Temporary source edits were restored, disposable test volumes removed, and the original development services restored with their existing data.

## Slice 2 Account access

**Deliverable:** Passkey registration and login with a protected, user-specific workspace.

**Related ADRs:** [Passkeys and sessions](adr/auth/001-passkeys-and-sessions.md), [Auth libraries and configuration](adr/auth/004-auth-libraries.md), [Account storage](adr/auth/005-account-storage.md), [Registration ceremonies](adr/auth/006-registration-ceremonies.md), [Registration session boundary](adr/auth/007-registration-session-boundary.md), [Passkey login](adr/auth/008-passkey-login.md).

- [x] **S2.1** Pin go-webauthn and SCS with pgxstore. Add auth constructors for discoverable, user-verified passkeys, exact configured origins, host-only HTTP-only `SameSite=Strict` cookies, a 12-hour absolute session lifetime, and five-minute server-enforced challenges. Derive the RP ID and Secure cookie setting from the browser origin; use localhost for both Compose modes. Document configuration and the library decision.
  - Verified generated registration/login options, challenge deadlines, emitted cookies, invalid configuration, HTTPS settings, and both Compose configurations. Go package tests pass; database integration checks were not run. Schema and registration wiring are complete in S2.2–S2.3; browser passkey flows remain for S2.10.
- [x] **S2.2** Add reversible migrations and generated sqlc models for UUID accounts with non-unique display names, RP-scoped WebAuthn user handles, one credential per account with typed metadata, and the SCS `pgxstore` sessions table with an expiry index. Update startup migration tests and document the storage decision.
  - Verified account/credential constraints, binary and metadata storage, pgxstore insert/update/expiry/delete behavior, and rollback/reapply on disposable Postgres databases. Pinned sqlc generation, dbmate up/down/up, and the full Go race suite with database integration pass. Applied the migration locally and verified app readiness after refreshing its Compose environment. Disabled unnecessary cleanup in the in-memory auth test store after race detection exposed a library cleanup race. Registration writes and transaction handling are complete in S2.4; login lookups and metadata updates are complete in S2.5.
- [x] **S2.3** Wire registration begin/finish routes and SCS with store cleanup before pool shutdown. Validate display names and bounded JSON requests, enforce the configured request origin, and keep browser-bound pending users and WebAuthn state in a bounded process-local store. Atomically consume expiring challenges before verification; begin replaces the browser's previous attempt and restart clears pending state. S2.4 extends verified finish with account persistence and authenticated-session creation.
  - Verified real WebAuthn attestation parsing, RP/origin/challenge/user-verification rejection, expiry/reuse/replacement, browser binding, restart behavior, capacity limits, and concurrent finish with exactly one success. Full Go race suite with database integration passes; focused race checks cover the final browser/restart cases. Live Vite proxy checks verified options, Postgres session-cookie round trips, origin rejection, and challenge consumption. Browser passkey UI verification remains in S2.10.
- [x] **S2.4** Persist the pending account and complete verified credential through generated sqlc queries in one pgx transaction. Replace anonymous SCS state with a new authenticated token and full lifetime, explicitly saving the session before returning 201 with account identity. Preserve single-use consumption, reject registration from authenticated sessions, and return safe errors for duplicate credentials, transaction failures, and session-store failures. Document the separate account/session commit boundary and passkey-login recovery path.
  - Full Go race suite with disposable Postgres integration passes. Verified account/credential rollback (including deferred commit failure), metadata round trips, duplicate display names, fresh-session state/deadline/token, old-cookie rejection, store failures without success responses or cookies, and one persisted account under concurrent finishes. Final focused race checks also cover non-default credential metadata. Login is implemented in S2.5; browser passkey UI remains in S2.6.
- [x] **S2.5** Add discoverable passkey login begin/finish with browser-bound, bounded, single-use challenges. Look up credentials by RP, credential ID, and user handle; restore complete credential metadata and serialize verification/updates under a database row lock. Persist counters, flags, clone warnings, and last-use time before issuing a fresh authenticated session. Share origin, response, and session helpers with registration.
  - Full Go race suite and disposable Postgres tests pass: signed assertions, invalid signatures/RP/origin/user handles, expiry/reuse/replacement/restart, capacity limits, concurrent finish, counter/backup-state updates, duplicate display names, transaction rollback, and session-store failures. Live Vite proxy checks verified discoverable options, session-cookie round trips, origin rejection, and challenge consumption. Browser passkey UI verification remains in S2.10.
- [ ] **S2.6** Build the login page with account creation, display-name entry, passkey prompts, loading states, cancellation handling, and useful error messages.
- [ ] **S2.7** Add a current-user endpoint, authentication middleware, frontend route protection, and origin protection for remaining state-changing requests. Registration and login share an exact-origin guard; reuse or adapt it for other mutations and WebSocket upgrades in Slice 4.
- [ ] **S2.8** Implement logout and session expiration. Establish the session invalidation boundary that terminal cleanup will use in Slice 4.
- [ ] **S2.9** Test protected-route unauthenticated access, logout invalidation, and session-expiry behavior after S2.7–S2.8. Registration challenge and persistence coverage is complete in S2.3–S2.4; login assertions, challenge handling, metadata updates, fresh sessions, and failure/concurrency coverage are complete in S2.5.
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
