# Commit registration before issuing an authenticated session

Status: Accepted

## Context

Verified registration must save the account and its credential together, then
replace the anonymous browser session. Database or session-store failures must
not grant authenticated access or produce a success response.

## Decision

After single-use challenge consumption and WebAuthn verification, use generated
sqlc queries within one pgx transaction to insert the pending account and the
complete verified credential record. Preserve binary identifiers, the public
key, raw flags, authenticator metadata, transports, and nested attestation and
extension records. Empty transports are stored as an empty array, not SQL NULL.

Only after that transaction commits, destroy the anonymous SCS session, clearing
its data and resetting its lifetime. Store the account UUID as `account_id` in a
fresh session. Explicitly commit that session before writing its new cookie and
returning HTTP 201 with the account ID and display name. The finish route loads
SCS state explicitly instead of using automatic response saving, which can write
a session-store error followed by the handler's success body.

Reject registration begin and finish with 409 when the loaded session already
contains an account ID. Registration must not silently switch an authenticated
browser to a different account. Duplicate account/credential constraints also
return 409. Other persistence or session-store errors return 503 without a new
authenticated cookie; responses do not expose database errors or credential data.

## Consequences

The account and credential transaction is separate from SCS session storage. If
the account commits but session deletion or saving fails, retain the account and
credential and tell the browser to sign in with the passkey. A lost database
commit acknowledgement can also leave the account committed; no authenticated
cookie is issued on that error. Do not retry the consumed challenge. Login is
the recovery path once its endpoint is implemented.

A successful session contains no pending-registration binding or other anonymous
state, has a new token and full configured lifetime, and identifies the account
by UUID. Future authentication middleware and session invalidation build on
that state; this decision does not implement login, logout, or resource access.

This supplements [account storage](005-account-storage.md) and
[registration ceremonies](006-registration-ceremonies.md).
