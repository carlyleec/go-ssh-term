# Attach a disposable xterm.js terminal to the workspace

Status: Accepted; implements the first terminal from
[the terminal workspace decision](002-terminal-workspace.md).

## Decision

Keep the page in its route file and its terminal panel/runtime under the route's
`-components` directory. Pin `@xterm/xterm` 6.0.0 and `@xterm/addon-fit` 0.11.0;
load their runtime and CSS only when the user opens a terminal.

The Connect picker opens host inspection. Unknown keys require explicit approval;
changed keys retain the explicit reset/reapproval flow. A verified host presents
**Open terminal**. The server independently rechecks trust when dialing.

Show one terminal panel with its destination snapshot and connecting, connected,
failed, or disconnected state. Disable another Connect until the panel is closed;
multi-terminal tabs remain the later workspace task. Editing/deleting a saved
configuration does not replace the active panel. A failed/ended shell retains
its output until the user closes the panel. Never reconnect automatically.

Derive the WebSocket URL from the browser origin and negotiate `ssh-terminal.v1`.
Only the server's `connected` status enables input. Encode keyboard/paste data as
UTF-8 bytes, preserve xterm binary input, and split input into 32 KiB messages.
Feed binary output directly to xterm. Validate bounded JSON status controls.
Use the fit addon's proposed dimensions and ResizeObserver to send bounded
resize controls, including an initial resize after shell readiness.

Keep 1,000 scrollback lines. Stop the transport with a visible message when the
browser's queued input or pending xterm output would exceed 1 MiB. These are
initial per-terminal browser bounds; aggregate server/session limits remain
later work.

Remove input listeners, socket handlers, and resize observation when transport
ends. Dispose xterm (including its addon) and close the socket on panel removal,
route unmount, session cleanup, or page departure. Guard asynchronous loading
and late events so Strict Mode or an unmounted panel cannot open another socket
or revive a completed state. The existing authenticated layout owns session
cleanup; no additional auth store or polling loop is introduced.

## Consequences

The first browser shell is wired without introducing tabs, restoration, audit
events, or a dedicated Reconnect action. [Server lifecycle bounds](../api/017-terminal-lifetime-and-io-bounds.md)
define Go registry cleanup on logout/shutdown and stalled-I/O handling.
Rendered tests use xterm/WebSocket fakes; native rendering, keyboard behavior,
and the development-proxy/bastion walkthrough remain explicit verification work.

References: [xterm addon integration](https://xtermjs.org/docs/guides/using-addons/)
and [stylesheet imports](https://xtermjs.org/docs/guides/import/).
