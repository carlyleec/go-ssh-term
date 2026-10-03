# Verify login and update credential metadata together

Status: Accepted

## Context

Discoverable passkey login must resolve the account without a unique display
name, preserve authentication metadata, and prevent simultaneous assertions
from overwriting newer counter values.

## Decision

Use discoverable WebAuthn login with required user verification. Bind its complete
SessionData to a random value in the initiating SCS session. Keep one pending
login per binding in a separate process-local map, bounded to 1,024 entries.
Prune expired entries on begin and atomically consume before parsing/verifying
finish. Replacement, expiry, failed finish, and process restart require a new
begin, as with registration.

Use generated sqlc queries to look up the account and credential together by
configured RP ID, credential ID, and user handle. Lock the credential row for
verification and metadata updates in a single pgx transaction. Reconstruct the
full stored credential and use go-webauthn's ValidatePasskeyLogin. Persist its
returned counter, clone warning, and raw flags, plus the last-use timestamp.
Unknown credentials and verification failures use the same generic 401 response.

Follow the library's counter policy: zero counters are valid, and a non-increasing
nonzero counter retains the prior count and records a clone warning. Treat that
warning as advisory; signature, challenge, origin, RP, ownership, user presence,
and user verification must still pass. Preserve the library's backup-state and
user-verification metadata semantics.

After metadata commits, replace anonymous state with a fresh authenticated SCS
session and full lifetime. Explicitly save the session before issuing its cookie
and returning the account ID and display name. Reject already authenticated
sessions with 409 instead of silently switching their identity. Share origin and
session helpers with registration; apply JSON/body limits and no-store responses.

## Consequences

Independent logins serialize credential updates while retaining independent
login sessions. A metadata transaction failure grants no new session. A later
session-store failure leaves verified metadata committed, issues no new cookie,
and requires a fresh login ceremony. Other sessions are unaffected.

The single-process challenge-store restriction remains. User-facing passkey
prompts, authorization middleware, logout, and terminal invalidation belong to
their remaining tasks.

This supplements [registration ceremonies](006-registration-ceremonies.md) and
[the session commit boundary](007-registration-session-boundary.md).
