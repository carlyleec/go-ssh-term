# Cancel terminal work on invalidation and bound stalled I/O

Status: Accepted; extends [the terminal registry](016-login-owned-terminal-registry.md)
and [logout](../auth/010-logout-and-session-expiry.md).

## Decision

Wire the shared dialer's invalidation callback to `Access.OnLogout`. After session
deletion, mark the login digest revoked under the registry mutex, cancel and close
all its attempts, and wait for their handlers to join their workers before the
callback returns. Other logins, including those for the same account, remain
live. Session-store failure does not invoke invalidation or report success.

Retain the revoked digest and its original expiry to reject an HTTP request that
passed authentication before logout but reaches the registry afterward. Admission,
client attachment, and shell publication check the same mutex-protected boundary.
Prune expired revocations during later admission/invalidation. No bearer cookie
or terminal content is retained. Existing absolute-deadline cancellation remains
independent of browser activity and session-store cleanup.

Track the browser transport immediately and attach SSH clients before PTY setup.
Closing entries stay in the registry until worker cleanup completes. This lets
logout and shutdown find attempts already closing, rather than mistaking removal
of an ID for completed resource cleanup. All network closes occur outside the
registry lock. Internal cleanup remains idempotent.

Stop all registry admission during server shutdown. Cancel/close existing attempts
and wait for their completion alongside HTTP draining under the configured
shutdown deadline. `net/http.Shutdown` alone does not wait for hijacked sockets.
Return a shutdown error if the deadline expires. Apply this cleanup on listener
failure as well as SIGINT/SIGTERM, before closing persistence resources.

| Operation | Bound |
| --- | --- |
| SSH dial through shell startup | 10 seconds total |
| WebSocket application/control writes | 5 seconds per write |
| WebSocket message body after its first frame header | 5 seconds, including continuation frames; pongs do not extend it |
| SSH stdin write or resize request | 5 seconds, then cancel/close the attempt |
| Output drain after SSH session completion | 5 seconds, then cancel/close the attempt |
| Browser heartbeat | Ping every 15 seconds; read silence expires after 45 seconds, renewed by data or pong processing |
| SSH heartbeat | `keepalive@openssh.com` every 15 seconds; any reply within 5 seconds establishes liveness |

WebSocket reads continue while awaiting SSH setup. Backpressure can pause reads
at one pending input message, but setup/input-operation deadlines then interrupt
the blocking work. Write failures cancel immediately so another output worker
cannot remain blocked behind a failed write. SSH keepalive requests are serialized
and bounded by closing the client on timeout. Ordinary idle shells remain open
when both peers answer; heartbeats never extend login expiry.

Cancellation closes registered transports immediately; handlers join workers
before reporting cleanup complete. SQLite's existing five-second external lock
wait can still delay a canceled setup's return. Transport loss and forced cleanup
may prevent final status delivery. No automatic reconnect is introduced.

## Consequences

Controlled peers cover logout/store failure, independent logins, pending setup,
late admission/publication, healthy/silent peers, incomplete messages, blocked
SSH input, slow browser output, and hijacked-connection shutdown. Native browser,
proxy, and real-bastion walkthroughs remain explicit verification work.

Audit events, reconnect/restoration behavior, multi-terminal UI, and aggregate
resource limits remain separate tasks. Revocation storage lives only in the
process; restarting destroys all live handles and pending attempts as well.
