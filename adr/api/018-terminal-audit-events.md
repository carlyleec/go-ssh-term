# Persist an audit trail for each terminal attempt

Status: Accepted; implements [audit events](006-audit-events.md) using
[connection snapshots](012-connection-storage.md).

## Decision

After WebSocket upgrade and registry admission, load one authorized configuration
snapshot and persist `start` before dialing. Start means an attempt, not a ready
shell. Use the registry's random terminal UUID as the attempt ID, with separate
random event UUIDs. Dial that same snapshot and recheck host trust and key ownership.
Inspection/approval, rejected HTTP upgrades, and requests rejected before an
owned snapshot is available do not create terminal audit events.

After closing transports and joining workers, write optional `failure` followed
by exactly one `end` in a transaction. All events retain the account, historical
configuration ID, destination name/host/port/username, and UTC nanosecond timestamp
from that attempt. Edits/deletion cannot change its destination history. A manual
reconnect is a new attempt with a fresh configuration lookup and UUID.

Persist the first observed application-owned failure code only:

- `host_trust_rejected`, `storage_unavailable`, `setup_timeout`, or `ssh_setup_failed`
  for setup failures (the last includes handshake, authentication, and PTY errors).
- `browser_transport_failed`, `ssh_transport_failed`, or `ssh_unresponsive` for
  transport/heartbeat failures.
- `terminal_io_failed`, `terminal_io_timeout`, or `output_drain_timeout` for I/O.
- `ssh_session_failed` for an unsuccessful remote session result.

Normal exit and intentional cancellation (browser close, logout, expiry, shutdown)
need only `end`. A concurrently observed failure can precede cancellation; these
codes describe observations, not a guaranteed root cause. Never persist or log raw
SSH/database errors, terminal contents, keystrokes, private keys, or login tokens.
The failure-code vocabulary stays internal; no HTTP or WebSocket schema changes.

## Storage failure and lifetime

Fail closed before dialing if `start` cannot be persisted, with a safe browser
message. Start writes share the ten-second setup budget. Completion writes use
an independent five-second context so logout/expiry cancellation does not erase
the final records. Close network resources before waiting for this database work;
registry completion includes this bounded attempt. SQLite's existing busy timeout
still applies. Shutdown retains its configured overall deadline.

If final persistence fails, roll back both final events and log only a fixed
message plus attempt UUID. Do not retry indefinitely or prevent logout from
completing. Storage outage or process crash can leave a start without an end;
this is incomplete history, not proof of a still-running shell. No recovery queue,
audit API, or audit-history screen is introduced.

## Verification

Controlled SSH/WebSocket tests cover normal/nonzero exit, trust rejection,
destination retention after edit/deletion, logout, refusal to dial when start
storage fails, and atomic rollback of final events. The existing race suite
continues to exercise stalled I/O, expiry, shutdown, and cleanup. Repeated shell
cycles verify distinct ordered attempt histories, while blocked replacement
authentication and concurrent close/invalidation/shutdown verify cancellation
without late publication or duplicate final events.
