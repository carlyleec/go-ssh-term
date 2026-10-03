# Stream terminals over WebSockets

Status: Accepted

## Context

Interactive terminals require continuous input, output, resize messages, and connection status in both directions.

## Decision

Use WebSockets between xterm.js and the Go gateway. Go bridges terminal traffic to SSH with a PTY. Use ordinary HTTP endpoints for account and saved-configuration operations. Do not split terminal input and output across HTTP POST and SSE.

## Consequences

A bidirectional transport fits the terminal interaction. The server must authenticate upgrades, validate origins, bound buffering and writes, and coordinate cancellation. The protocol must distinguish terminal data from resize and status messages.
