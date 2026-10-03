# Invalidate the current login session and observe expiry

Status: Accepted; extends [session and route protection](009-current-user-and-route-protection.md).

## Context

Logout must remove server authority, not merely hide workspace UI. SCS already
enforces the absolute 12-hour deadline on session lookup, but an open workspace
also needs to notice expiration. Future terminal ownership needs an identity
scoped to one login session rather than one account.

## Decision

POST /api/auth/logout requires the configured exact Origin and JSON content type.
Load SCS state explicitly, destroy the session, then expire the cookie and return
204 with no body. Do not commit a replacement anonymous session. Logout is
idempotent for missing, anonymous, expired, and previously deleted sessions and
does not require an account lookup. Store failures return 503 without clearing
the cookie or claiming success. Other login sessions remain valid.

Protected handlers receive LoginSession through SessionFromContext, containing a
SHA-256 digest of the bearer token and the absolute ExpiresAt deadline. Neither
value is sent to the frontend. Access.OnLogout is an optional startup-configured,
synchronous, concurrency-safe and idempotent callback invoked after deletion and
before success. It is the integration point for invalidating a terminal owner.

When terminals are introduced, their registry must enforce the supplied deadline
without relying on HTTP polling or database cleanup, and coordinate the logout
callback with in-flight connection publication. The callback alone does not solve
that race. There are no terminal resources or per-session timers to manage yet.

The workspace observes the current-user query every 30 seconds while visible,
and refetches when the document becomes visible or the network reconnects.
Confirmed 401 responses remove private UI, cancel pending queries, clear cached
queries/mutations, and navigate to login. Network/server errors show a retry
screen without treating the session as logged out. Browser timer throttling can
delay UI updates in background tabs; server authorization always checks expiry.

The logout mutation only makes the request. Its onSuccess handler clears cached
data and navigates after confirmed server success. A separate effect handles
session loss discovered by the current-user query. Neither path retries logout
automatically or queues it for a later reconnect.

## Consequences

Logout affects only the current login session and never changes the passkey.
Concurrent authenticated requests already admitted before logout may complete;
future terminal creation must enforce the documented registry boundary. A lost
logout response is safe to retry. Multi-tab coordination remains outside v1;
other open workspaces discover invalidation through their session checks.
