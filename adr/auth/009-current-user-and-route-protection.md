# Verify server sessions before granting workspace access

Status: Accepted

## Context

Passkey registration and login establish server-side sessions. Subsequent API
requests and browser navigation need a shared way to verify the current account.
A successful past login or a cached frontend identity cannot authorize requests.

## Decision

Expose GET /api/auth/me with an account object containing only id and display_name.
Require a valid SCS session containing an account UUID and a persisted account
matching the configured RP. Missing, anonymous, expired, malformed, or unknown
sessions and missing accounts return 401. Session-store and account-lookup failures
return a safe 503. Responses are no-store and vary by Cookie. Reading identity
neither creates a cookie nor refreshes the absolute session deadline.

Use Access.Require for protected API handlers. It loads and verifies the session
and attaches the account to request context. Resource handlers must still scope
queries to that account ID. Require exact configured Origin for unsafe methods
and WebSocket upgrades; keep this reusable guard separate from the JSON content
requirement used by registration and login. There are no other mutation endpoints
yet; future handlers must use this boundary.

Share one QueryClient between React and TanStack Router. Before entering the
connections route, fetch current-user data with zero stale time and no automatic
retry. Redirect only a confirmed 401 to login; service and network failures show a
retry screen. Redirect signed-in users away from login. After a successful passkey
ceremony, discard prior current-user query state and navigate to the workspace,
which checks the server again. Do not persist account identity in browser storage.

## Consequences

The workspace page is private and displays the verified account name. Go may
still serve the public SPA shell at /connections; client redirects govern UI,
while server middleware enforces API access. Active-page session expiry and logout follow
[the lifecycle decision](010-logout-and-session-expiry.md). Terminal cleanup
remains part of the terminal slices.
