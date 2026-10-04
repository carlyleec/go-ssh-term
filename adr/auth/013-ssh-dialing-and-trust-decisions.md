# Verify host identity before SSH user authentication

Status: Accepted; implements [explicit host trust](003-host-verification.md).

## Context

Browser approval must identify the exact endpoint and key inspected. The remote
server or saved configuration can change while a user considers the fingerprint.
No open SSH connection or database transaction should wait for that decision.

## Decision

Use the already-pinned `golang.org/x/crypto/ssh` for direct SSH dialing. Apply a
ten-second setup context and socket deadline to TCP connection and SSH handshake;
cancellation closes the socket. Existing SQLite lock-wait bounds still apply to
database operations. Remove the socket deadline on successful handoff. The caller
owns the returned client and configuration snapshot, including later session
invalidation, terminal closure, and shutdown cleanup.

Inspect hosts by completing cryptographic key exchange and aborting in the host
key callback, before user authentication to the inspected endpoint. For a target behind a
bastion, inspection first verifies and authenticates the bastion as described in
[single-jump transport](../api/020-single-jump-transport.md). Display SHA-256 fingerprints and
the normalized host/port. Offer Ed25519, ECDSA, and RSA SHA-2 host signatures;
prefer the trusted key's algorithm when available. Do not use an insecure host-key
callback, SSH certificates, password authentication, or automatic key replacement.

Expose authenticated, exact-Origin-protected POST operations under
`/api/connections/{id}`:

- `/host-key` returns the presented fingerprint, key algorithm, normalized endpoint,
  trust state (`unknown`, `trusted`, or `changed`), and prior trusted fingerprint.
  [Hop-aware decisions](../api/021-hop-trust-and-audit.md) extend this response
  with the inspected hop and guide bastion approval before target inspection.
- `/host-trust` accepts `host`, `port`, and the displayed `fingerprint`. Probe
  again and require that exact fingerprint. In a short transaction, recheck the
  owned configuration's endpoint and insert trust only if absent. Existing trust
  must match; changed keys return 409 and are never overwritten.
- `/host-trust/reset` accepts the endpoint and the **previously trusted**
  fingerprint. Recheck configuration and stored trust in a short transaction,
  then remove only that matching record. Reset neither approves the replacement
  nor touches an already-established connection. It affects all configurations
  for this account and endpoint. A subsequent connection requires fresh approval.

Rejecting the browser prompt simply closes it and saves nothing. Decision bodies
are bounded JSON; errors use safe envelopes without raw SSH errors. Inspection and
approval do not load the inspected endpoint’s private key or attempt login to
that endpoint; traversing a bastion requires its configured credentials. Each actual dial rechecks
the server key against stored trust during key exchange, then loads only the
owning account's encrypted key. Clear decrypted file bytes after parsing the
signer. In-memory signer material remains subject to Go's memory management.

## Consequences

First contact and approval use separate short connections. Repeated approval of
the same key is safe; stale destination/fingerprint decisions fail. Trust is per
account/endpoint and survives saved-configuration deletion. DNS still determines
the reachable address, while the approved host key establishes server identity.
Initial trust depends on the user's fingerprint comparison.

The picker verifies the host before offering **Open terminal**. The backend
[terminal path](../api/015-direct-terminal-shell.md) attaches a PTY and shell
after dialing, with [login-owned handles and expiry](../api/016-login-owned-terminal-registry.md).
[Lifetime management](../api/017-terminal-lifetime-and-io-bounds.md) handles
logout, shutdown, and stalled peers. [Terminal audit events](../api/018-terminal-audit-events.md) record connection attempts; trust decisions
are not themselves terminal connection events.
