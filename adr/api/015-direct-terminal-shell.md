# Attach one direct SSH shell to each terminal socket

Status: Accepted; extends [terminal transport](014-terminal-websocket-protocol.md)
and [verified SSH dialing](../auth/013-ssh-dialing-and-trust-decisions.md).

## Context

The terminal transport needs a working SSH path before the tab UI and shared
login-session registry are added. A successful upgrade alone does not establish
host trust, authenticate to SSH, or mean a shell is ready.

## Decision

After upgrade, send `connecting` and call the existing owner-scoped dialer. It
reloads the saved configuration and verifies current host trust before loading
the user's signer. Unknown or changed keys fail safely; the socket never grants
trust. Approval/reset remains in the existing HTTP flow.

Open one SSH session, request an `xterm-256color` PTY at 80 columns by 24 rows
with echo enabled, and start the user's interactive shell. Use the ten-second
setup budget across dialing, channel creation, PTY allocation, and shell startup.
Cancellation or timeout closes the SSH client to interrupt blocking requests.
Remove the setup deadline only after the shell request succeeds. Browser resize
messages subsequently set the real dimensions using SSH `window-change`.

Send `connected` before forwarding output. Copy input bytes to SSH stdin and
forward stdout and stderr as binary messages, splitting output into at most
32 KiB chunks. A socket mutex serializes output and status writes. Read the
browser in a separate goroutine, with at most one message pending delivery to
the input worker. Backpressure does not create an unbounded application queue.
SSH stdout and stderr retain their separate stream ordering; their relative
interleaving is unspecified.

After remote exit, drain output before sending `disconnected` and closing
normally. Failed setup sends `failed` with safe text and closes normally. Never
forward raw SSH errors, execute a server-side command in place of the shell,
record terminal contents, or reconnect automatically.

The handler owns its WebSocket, SSH client, and forwarding workers. Observed
browser closure cancels setup or the shell. Returning closes both transports
and joins the workers; setup failure and remote exit likewise release resources.
Editing/deleting the saved configuration does not mutate a running SSH client.

## Consequences

The backend terminal endpoint now opens a real SSH shell after explicit trust.
Controlled SSH/WebSocket peers verify the protocol and resource handoff. Native
browser/xterm.js and Docker-bastion walkthroughs remain separate verification.

The [login-owned registry](016-login-owned-terminal-registry.md) tracks pending
and live handles and enforces absolute expiry. [Lifecycle bounds](017-terminal-lifetime-and-io-bounds.md)
cover logout, shutdown, heartbeats, and stalled I/O. [Terminal audit events](018-terminal-audit-events.md) record attempt outcomes. The
[browser terminal](../frontend/008-first-browser-terminal.md) is wired separately;
no restoration or automatic reconnect is introduced.
