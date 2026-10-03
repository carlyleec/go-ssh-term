# Browser SSH Gateway PRD

Build a small, locally runnable portfolio project demonstrating Go networking, concurrency, authentication, and resource management. An evaluator should be able to start Docker Compose, create an account with a passkey, import the supplied demo credentials and SSH configuration, and open terminals on a bastion and private hosts.

## Product and architecture

- Go API serving the built React frontend from the same origin.
- React with TanStack Router, Tailwind CSS, and daisyUI.
- xterm.js with WebSockets for interactive terminal input and output.
- WebAuthn passkeys for authentication and server-side login sessions.
- Postgres for persistent application data, backed by a Docker volume.
- Docker Compose for the application, database, and Ubuntu targets running OpenSSH.
- One bastion and two private targets. The gateway can reach the bastion directly; private targets require an SSH connection through the bastion.

The public and private networks are local Docker lab networks, not a public deployment. The gateway must not share the private target network. Users may also configure other SSH hosts reachable from the gateway container.

## Concepts and navigation

| Concept | Meaning |
| --- | --- |
| Account | A user identified internally by a stable ID, with a display name and one passkey. |
| Login session | The authenticated browser session and authority for its live SSH connections. |
| Saved connection | A user-owned host configuration that persists independently of running terminals. |
| SSH key | A named, user-owned private key reusable across saved connections. |
| Live SSH session | An active SSH connection and shell, owned by a login session and represented by a terminal tab. |

Routes are `/` for the landing page, `/login` for sign-in and account creation, and `/connections` for the protected workspace. A separate connection detail route is not required for v1. Registration may share the login page rather than requiring another route.

The workspace offers Import SSH config, Add connection, key management, and Connect actions. Forms and the connection picker use modals. An empty workspace explains the demo setup and offers Connect prominently. Each successful connection opens a terminal tab; multiple tabs may use the same saved connection. Only one terminal is visible at a time, while other tabs stay connected.

## Slice 1 Landing page and runnable application

**Outcome:** An evaluator can start the app and understand what to try, and a developer can iterate without rebuilding containers for every edit.

Deliver Compose setup, Go serving React, a database connection and migration mechanism, and a landing page with an entry point to login. Docker builds the frontend so evaluation does not require a local Node or Go installation.

### Local development

Provide a development Compose override with source directories mounted into containers. Run Go under Air for automatic rebuilds and process restarts, and React under Vite for frontend hot updates. Postgres and the SSH lab use the same services and network topology as the demo; the lab becomes available as its slices are implemented.

Developers open one documented localhost URL served by Vite. Vite proxies API requests and terminal WebSockets to Go, keeping browser requests on one origin. Configure WebAuthn and origin checks for that browser-facing URL. The normal demo continues to serve compiled React assets directly from Go without development servers.

An Air restart ends live SSH sessions; it does not preserve running shells. Persistent accounts, keys, and saved connections remain available. Frontend hot updates are a development convenience, not a guarantee of terminal-state preservation.

The README documents commands to start each mode, view logs, run migrations and tests, and stop containers without removing data. Destructive volume reset is a separate, explicitly labeled command. Pin development tools and keep build output and dependency caches out of source control. Commit generated route and query source alongside their inputs.

**Acceptance criteria**

- After the documented migration setup, `docker compose up --build` starts the application and Postgres from a fresh checkout.
- The landing page explains the project and links to account access.
- Direct navigation and refresh work for React routes; unknown API paths do not return the SPA HTML.
- Persistent database data survives ordinary container recreation.
- No external identity provider or hosted service is needed.
- Editing Go source triggers an Air rebuild and restart; build errors appear in developer logs.
- Editing React or styles updates the browser through Vite without rebuilding the application image.
- Development mode preserves private-network isolation and uses persistent database and encryption-key volumes.
- As auth and terminals are added, passkey flows, cookies, and terminal WebSockets work through the development proxy without disabling origin checks.

## Slice 2 Account access

**Outcome:** A user can create an account and access a private workspace.

Account creation asks for a display name and enrolls a passkey. Login uses that passkey without a password. Store the account, public credential information, and login sessions. Display names are labels, not authorization identifiers. There is one passkey per account and no recovery flow in v1.

**Acceptance criteria**

- Registration and login work through the documented localhost URL, with clear cancellation and failure messages.
- WebAuthn challenges expire and cannot be reused; configured origin and relying-party checks are enforced.
- Unauthenticated users cannot access workspace APIs or establish terminal WebSockets.
- Session cookies are HTTP-only with appropriate same-site and transport settings; state-changing requests and WebSocket upgrades enforce origin protections.
- Logout invalidates the current login session and closes all SSH connections it owns. Other login sessions are unaffected.
- Session expiry also ends its live connections. Exact timeout values are implementation choices to document.
- Users cannot access another user's keys, configurations, host trust decisions, or terminals by guessing identifiers.

## Slice 3 SSH key management

**Outcome:** A user can upload credentials once and reuse them.

Provide a key-management modal for uploading, naming, listing, and deleting SSH keys. Persist encrypted private keys in Postgres. Generate an application encryption key on first initialization and persist it in a separate Docker volume; keep it out of the database and repository. Decrypt SSH keys on the server only when needed.

**Acceptance criteria**

- Valid private keys without passphrases can be uploaded; invalid and passphrase-protected keys receive clear errors.
- List responses expose identifying metadata, never stored private-key contents.
- A key remains usable after container restart while both persistent volumes remain intact.
- Deletion is blocked while a saved connection references the key.
- Missing encryption material produces an actionable error instead of silently replacing the key and making existing data unreadable.
- Documentation explains that losing the encryption-key volume requires re-uploading SSH keys, and that access to both volumes defeats this protection.
- The repository includes a clearly labeled demo-only private key whose public key is authorized on the lab hosts.

## Slice 4 Saved connections and a first terminal

**Outcome:** A user can save a direct connection and use a real shell on the bastion.

Add and edit forms capture a name, hostname, port, username, and uploaded key. Persist configurations per user. Connect opens a picker; selecting a configuration opens an SSH shell with a PTY in xterm.js. Add the Docker bastion target in this slice.

**Acceptance criteria**

- Saved connections can be created, listed, edited, and deleted. Editing affects future sessions; deleting a configuration does not close an existing terminal.
- First contact displays the SSH host fingerprint for approval before authentication proceeds. Trust is stored per user and host endpoint.
- A changed host key blocks connection with a clear explanation; no silent trust bypass is allowed. A way to explicitly reset trust must be documented.
- Typing, output, shell control keys, and terminal resizing work.
- Connection progress, authentication failure, unreachable hosts, and terminal closure are visible in the UI.
- Closing a terminal tab ends its SSH connection and releases resources. Typing `exit` or losing the remote SSH connection leaves a disconnected tab with a manual Reconnect action.
- The bastion contains a readable `/host-info.txt` identifying it as the bastion host.
- Connection start, end, and failure events are recorded with user, destination, time, and relevant non-secret failure information. Do not log terminal output, keystrokes, private keys, or authentication secrets.

## Slice 5 Bastion access to private targets

**Outcome:** A user can reach private hosts through a saved bastion configuration.

Add an optional Jump through selection to saved connections. It references another connection owned by the same user. Support exactly one jump, using the selected key for each hop. Add two Ubuntu OpenSSH targets on the private Docker network.

**Acceptance criteria**

- The gateway cannot directly reach either private target; each is reachable through the bastion.
- Both bastion and target undergo host-key verification.
- Invalid references, self references, and multi-hop chains are rejected.
- A referenced bastion configuration cannot be deleted until dependent configurations are updated or removed.
- Each private target has a readable `/host-info.txt` identifying that host.
- Failed or closed sessions release both the target connection and the resources used for its jump. Closing one terminal does not break another.

## Slice 6 SSH config import

**Outcome:** A user can import the supplied lab configuration instead of entering each host manually.

The import modal parses a supported subset of SSH config, previews discovered connections, lets users choose entries, maps identity references to uploaded keys, and confirms before saving.

Supported directives are `Host`, `HostName`, `User`, `Port`, `IdentityFile`, and a single `ProxyJump`. This is a limited importer, not full OpenSSH configuration compatibility.

**Acceptance criteria**

- A sample configuration maps to the Compose bastion and private targets and imports successfully.
- `IdentityFile` is a mapping hint only. The app never reads client or server filesystem paths specified in uploaded config.
- Jump references resolve to selected or existing user-owned configurations before saving.
- Unsupported directives or syntax, including `ProxyCommand`, are reported explicitly. No imported commands are executed.
- Preview identifies naming conflicts and unresolved keys or jumps; confirmation never silently overwrites existing configurations.
- No configurations are persisted until the user confirms a valid selection.

## Slice 7 Multiple terminals and connection lifecycle

**Outcome:** A user can work across terminal tabs and recover from browser interruption with fresh shells.

Support concurrent SSH sessions, including several sessions to the same saved host. The login session is the source of authority for its live connections. Keep running SSH clients in Go memory; database identifiers do not preserve network connections.

**Acceptance criteria**

- Switching terminal tabs preserves running sessions and does not mix their input or output.
- Refresh or browser-network interruption cleans up old connections and creates fresh SSH sessions for terminals that were active, while the login session remains valid.
- Reconnection starts new shells. It does not preserve commands, working directories, terminal output, or full-screen programs; the UI explains this behavior.
- SSH-side disconnects, explicit tab closure, and `exit` do not trigger automatic reconnection. Failed replacement connections offer manual retry rather than an endless loop.
- Logout invalidates restoration state, closes every connection owned by that login session, and prevents an in-flight reconnect from opening another.
- Navigating away or closing the browser does not leave SSH resources running indefinitely. Disconnect detection and cleanup have documented bounds.
- Slow or disconnected browsers cannot cause unbounded output buffers or leaked goroutines. Sessions are cleaned up on server shutdown; recovery across server restarts is not promised.
- Tests cover ownership checks, concurrent terminals, disconnect cleanup, and logout racing with connection creation or reconnection.

## Completion criteria

An evaluator can follow the README to start Compose, register with a display name and passkey, upload the demo key, import the sample config, approve host fingerprints, and open multiple terminal tabs. They can read each host's identifying file, reach private targets only through the bastion, close terminals, and log out. Saved data survives container recreation when volumes are retained.

Each slice includes its UI, Go behavior, persistence changes where needed, and focused verification. Audit events begin with the first terminal slice rather than being postponed until the end. Favor direct code and standard Go facilities over generic service or repository frameworks.

## Exclusions and open questions

Out of scope: public hosting, production hardening claims, password login, account recovery, multiple passkeys per account, passphrase-protected SSH keys, password-based SSH authentication, multiple jump hops, full SSH config compatibility, split-pane terminals, file transfer, terminal recording, an audit-history screen, and resuming existing shells after disconnect or server restart.

Multiple browser tabs normally share a login-session cookie within the same browser profile. Coordinating their terminal workspaces, restoration behavior, and simultaneous control is explicitly deferred beyond v1; this PRD does not require synchronization or enforcement of a single browser tab. Server-side ownership and logout guarantees still apply.

Implementation details to settle within the relevant slices: the exact accepted SSH key formats and `Host` syntax, timeout and resource limits, explicit host-trust reset flow, and the bounded detection and ordering used to replace disconnected sessions.
