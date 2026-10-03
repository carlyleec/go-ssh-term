# Configure go-webauthn and SCS

Status: Accepted; Postgres-specific storage is superseded by [011 SQLite account persistence](011-sqlite-account-persistence.md). Authentication and session guarantees remain in effect.

## Context

Passkey verification and browser sessions need maintained libraries that fit the
existing Go HTTP server and shared pgx pool. This supplements
[the passkeys and sessions decision](001-passkeys-and-sessions.md).

## Decision

Use `github.com/go-webauthn/webauthn` for passkey ceremonies and
`github.com/alexedwards/scs/v2` with its `pgxstore` adapter for persistent sessions.
Pin dependencies in `go.mod`; review WebAuthn migration notes when upgrading.
The adapter owns session queries; dbmate owns its schema migration. Application
account and credential queries continue to use sqlc.

Derive the relying-party ID from the configured browser-origin hostname. Use
`localhost` for both demo (`http://localhost:8080`) and development
(`http://localhost:5173`), allowing only the selected mode's configured origin.
Reject IP origins and non-localhost HTTP origins. Other domains require HTTPS.
Keep Docker's published port bindings on IPv4 loopback.

Require discoverable credentials and user verification. Request no attestation
and leave authenticator attachment unrestricted. Login uses passkey account
selection, not display-name lookup.

Default login sessions to a 12-hour absolute lifetime without an idle timeout,
configured through `SESSION_LIFETIME`. Use a persistent `ssh_term_session` cookie
with `HttpOnly`, `SameSite=Strict`, path `/`, and no Domain. Set `Secure` for HTTPS;
local HTTP development and demo cookies omit it. Strict same-site cookies fit the
local passkey flow, which has no external authentication redirects. External
links omit the cookie on the initial navigation; the public React shell can
load and make same-site authenticated API requests. Request-origin checks remain
required for state changes and WebSocket upgrades. Require a fresh authenticated
session after registration or login.

Default both ceremony lifetimes to five minutes through `CHALLENGE_LIFETIME`,
with server-side timeout enforcement. Pending WebAuthn `SessionData` is distinct
from authenticated login state. Bind it to the initiating browser and consume it
atomically, including concurrent finish attempts; SCS get/remove alone does not
provide this guarantee.

## Consequences

Changing the RP hostname changes the credential scope. Use `localhost`
consistently and update old IP-based environment overrides. Exact origin checks
still distinguish the demo and Vite ports.

SCS manages HTTP sessions, not active SSH connections. Application logout and
expiry handling must invalidate the owning login session and reach the terminal
registry. Database cleanup is housekeeping, not a terminal-expiry notification.
Without an idle timeout, terminal traffic need not refresh HTTP session activity.

Auth constructors precede schema and endpoint implementation. Wire them after
the session migration exists, and stop the store cleanup before closing the pool.
