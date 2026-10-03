# Bind registration challenges to browser sessions

Status: Accepted

## Context

WebAuthn verification needs the exact pending user and ceremony data returned by
begin. Removing values from an SCS request-local session does not prevent two
concurrent requests from reading and using the same challenge.

## Decision

Keep pending registrations in a process-local map keyed by a random 32-byte
binding stored only in the browser's server-side SCS session. Pending state holds
a new random UUID account ID, a separate random 32-byte WebAuthn user handle, the
display name, and the complete library SessionData. None of this grants login
access or creates an account.

Allow one pending registration per binding. Begin replaces the previous attempt.
Bound the map to 1,024 entries and prune expired entries on begin. When capacity
is exhausted, reject new bindings with 503. Consume pending state under a mutex
before verification, including malformed or failed finish attempts. A missing
or expired session, or a different browser session, cannot use the pending state.
Require a fresh begin after failure, expiry, replacement, or process restart.
The store has no background cleanup goroutine; expired entries remain bounded
and are never usable.

Both registration POST routes require the exact configured Origin and JSON
content type before loading SCS state. Responses use Cache-Control: no-store.
Begin accepts a display_name label, trims surrounding whitespace, and permits
1–64 Unicode code points without control characters. Limit begin bodies to
4 KiB and finish bodies to 64 KiB. Verify finish with go-webauthn, including RP,
client origin, challenge, and user-verification requirements.

## Consequences

This provides atomic single-use consumption within the single Go process used
by the local app. Multiple backend processes would require a shared atomic
challenge store. Air restarts invalidate unfinished ceremonies; SCS sessions
remain in Postgres. These constraints are acceptable for the local demo.

The registration origin guard is available now; authentication middleware and
origin protection for other mutations and WebSocket upgrades remain separate.
After verification, account persistence and fresh-session creation follow
[the registration session boundary](007-registration-session-boundary.md).
The browser registration UI must wait for the complete account-access slice.
