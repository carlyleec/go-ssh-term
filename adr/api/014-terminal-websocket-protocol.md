# Bound and authenticate the terminal WebSocket transport

Status: Accepted; extends [WebSocket transport](003-websocket-terminal-transport.md).

## Context

Terminal bytes must survive arbitrary SSH output and control keys. Resize and
status messages need a distinct, small contract. Authentication and ownership
errors must fail before the HTTP connection is upgraded.

## Decision

Pin `github.com/gorilla/websocket` v1.5.3. Register
`GET /api/connections/{id}/terminal` directly on the API ServeMux. This streaming
endpoint is an exception to [Huma HTTP operations](011-huma-and-generated-contracts.md):
it hijacks the connection rather than serializing a response. Its wire contract
is recorded here, outside the generated OpenAPI HTTP types and Zod schemas.

Require the exact configured Origin on every request to the endpoint, including
malformed handshakes. Reuse `Access.Require` to verify the cookie, account, and
login session; look up the canonical saved-connection UUID under that account
before upgrade. Unknown and other-account IDs both return 404. Missing/expired
sessions return 401, origin failures 403, storage failures 503, and invalid
handshakes/subprotocols 400. Handler errors use safe `{ "error": "..." }`
bodies. The mux rejects unsupported methods with 405. Reads never renew cookies.

Clients must offer `ssh-terminal.v1` in `Sec-WebSocket-Protocol`; the server must
select it. There is one saved destination per socket, no client-supplied account
or live-session ID, and no multiplexing. Compression stays disabled.

| Direction | Message | Limit |
| --- | --- | --- |
| Both | Binary: raw terminal bytes, including control keys and non-UTF-8 output | 32 KiB per message |
| Browser → Go | UTF-8 text: `{"type":"resize","cols":80,"rows":24}` | 1 KiB; integer dimensions 1–1000 |
| Go → browser | UTF-8 text: `{"type":"status","state":"connecting"}` | 1 KiB; optional safe `message` up to 256 UTF-8 bytes |

Status states are `connecting`, `connected`, `failed`, and `disconnected`.
`connected` means the shell is ready, not merely that the WebSocket opened.
`failed` describes setup failure; `disconnected` describes the end of a shell.
The shell handler sends a final status before closing when possible.
Transport loss may prevent final status delivery. Neither failure nor closure
itself requests automatic reconnection.

Text from the browser accepts only resize messages. Reject malformed JSON,
unknown fields/types, trailing JSON values, invalid UTF-8, and out-of-range
dimensions with close 1008. Oversized messages close with 1009; limits cover
the complete message across continuation frames. Binary bytes are never decoded
as JSON or strings. Clients split large input/pastes into messages within the
limit; SSH output writers likewise use bounded chunks. Empty binary messages
are valid no-ops. WebSocket ping/pong and close frames keep their standard meaning;
explicit terminal close uses a WebSocket close, not a JSON command.

Use one reader per socket and serialize data/status writes with a socket mutex.
Read bounded payloads without output queues; bound each application
write, close-frame write, and upgrade handshake to five seconds. Never log
terminal payloads or return raw SSH/storage errors in status messages.

## Consequences

The endpoint attaches one [direct SSH shell](015-direct-terminal-shell.md) after
verified dialing. Transport helpers are exercised with loopback peers.
Login-session registry/expiry/logout enforcement after upgrade, complete bounded
I/O cleanup, shutdown, heartbeat detection, auditing, and browser UI remain
in the following terminal tasks.

Library behavior: [Gorilla WebSocket documentation](https://pkg.go.dev/github.com/gorilla/websocket).
