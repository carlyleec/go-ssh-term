# Give each target transport its own verified SSH jump

Status: Accepted; implements the transport portion of [one SSH jump](005-ssh-configuration.md)
and extends [saved jump connections](019-saved-jump-connections.md).

## Decision

Re-read the selected bastion as an owned, direct connection before opening it.
Verify its host key against account/endpoint trust, then authenticate with its
configured encrypted key. Open a `direct-tcpip` channel to the target host/port;
the bastion resolves the target name. Never fall back to direct target TCP access.
Run the target's SSH handshake over that channel, independently verifying its
host key and authenticating with its own configured key. Private keys remain in
the gateway; neither agent forwarding nor copying keys to the bastion is needed.

Each target connection owns a dedicated bastion client. Closing the target
transport closes the bastion first to interrupt blocked channel writes, then
closes the channel. Existing terminal invalidation, expiry, shutdown, heartbeat,
and setup-failure cleanup therefore close the whole chain without disrupting
another terminal.

One setup context bounds both hops and terminal shell creation. Forwarded channels
do not support socket deadlines. Context cancellation closes the owning bastion
to interrupt channel opening and the target handshake. Open channels synchronously
under that cancellation hook to avoid leaving a pending channel-open goroutine.
The successful transport handoff removes setup cancellation hooks; the terminal
lifecycle owns subsequent cleanup.

Target inspection/approval authenticate to an already-trusted bastion but abort
before authenticating to the target. No SSH connection or database transaction
waits for browser approval. Trust remains scoped to account and endpoint.

## Consequences and verification

The current UI requires approving the bastion separately before inspecting its
target. Guided per-hop fingerprint prompts, error attribution, and audit snapshots
remain subsequent work. The terminal wire protocol and database schema do not change.

Controlled SSH peers verify separate credentials, target names resolved through
forwarding, independent host-key rejection, shell input/output, isolated terminal
closure, logout cleanup, rejected forwarding, canceled channel opening, and a
stalled target handshake. Real Docker/browser verification remains a separate
integration check.
