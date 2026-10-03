# Record connection events without terminal contents

Status: Accepted

## Context

Connection history helps demonstrate lifecycle behavior and diagnose failures, while terminal recording would add substantial data handling and scope.

## Decision

Persist connection start, end, and failure events with user, destination, timestamp, and relevant non-secret failure information. Exclude terminal output, keystrokes, private keys, and authentication secrets. Do not build an audit-history screen in v1.

## Consequences

Audit data supports connection-level inspection through Postgres but cannot reconstruct commands or terminal activity. Event writing is part of connection lifecycle handling, starting with the first terminal slice.
