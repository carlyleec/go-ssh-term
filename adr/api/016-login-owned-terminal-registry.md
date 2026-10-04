# Bind live terminal handles to verified login sessions

Status: Accepted; extends [connection lifecycle](004-connection-lifecycle.md),
[direct shells](015-direct-terminal-shell.md), and
[login-session identity](../auth/010-logout-and-session-expiry.md).

## Context

Saved configurations belong to accounts, but live shells belong to one login.
Separate logins for the same account must not control each other's terminals.
Several sockets may also open the same saved configuration independently.

## Decision

Keep one process-local registry on the application's shared dialer. Allocate a
fresh UUID per terminal attempt, separate from the saved-connection UUID. Capture
the verified account ID and `auth.LoginSession` from request context. Never take
ownership claims from WebSocket messages, expose the session digest/deadline,
or persist runtime handles.

Register pending setup before dialing, then publish the SSH client, session, and
stdin handle together after shell startup and before `connected`. The registry
rejects duplicate publication and publication after cancellation or expiry.
Use a mutex for map/handle changes; never hold it while doing network I/O.

Every browser input, resize, and close operation checks the terminal ID against
the captured account, login-session digest, original deadline, and active
context. An unknown, foreign, expired, or removed ID returns the same internal
error. Another login for the same account receives no special access. Operations
already admitted may race with closure, which interrupts them by closing SSH.
Output/status workers remain bound to their original socket and cannot select
another terminal; they require no client-directed lookup.

Enforce the absolute login deadline with the attempt's context, including during
setup and while idle. Expiry cancels pending work and removes/closes live handles
without depending on browser polling or database cleanup. Removal is idempotent
for internal cleanup and checks the exact entry. Worker cleanup still joins in
the owning handler. The registry lifetime is independent of saved-configuration
edits/deletion; closing one terminal leaves all other entries alone.

## Consequences

There are no new HTTP endpoints or wire fields. The existing socket selects one
server-owned terminal; reconnecting creates a distinct attempt. The registry
holds no terminal contents, private-key bytes, or bearer cookies.

Logout invalidation, cancellation/publication races with logout, coordinated
server shutdown, and remaining stalled-I/O cleanup are still lifecycle work.
Registry checks do not reload the session store per keystroke. Deadline expiry
may end the transport without a final status message. Restoration, heartbeat
detection, aggregate resource limits, and multi-tab coordination are unchanged.
