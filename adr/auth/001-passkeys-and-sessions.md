# Authenticate with passkeys and server-side sessions

Status: Accepted

## Context

The local app needs separate user-owned resources without passwords or an external identity provider.

## Decision

Allow local account creation with a display name and one WebAuthn passkey. Use stable account IDs and server-side login sessions represented by HTTP-only cookies. Validate WebAuthn challenges and browser origins, and protect state-changing requests and WebSocket upgrades.

Enforce ownership on keys, saved connections, host trust, and live sessions. Logout invalidates the current login session and closes its terminals without affecting other login sessions. Recovery and additional passkeys are outside v1.

## Consequences

Passkeys establish identity; login sessions authorize subsequent requests. Display names do not grant access. Losing access to the passkey requires a new account. Expiry and logout must reach active SSH sessions as well as HTTP handlers. Library selection and lifetime values remain open.
