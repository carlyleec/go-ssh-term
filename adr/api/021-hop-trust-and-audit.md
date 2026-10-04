# Guide trust decisions and retain audit context for both SSH hops

Status: Accepted; extends [host decisions](../auth/013-ssh-dialing-and-trust-decisions.md),
[audit events](018-terminal-audit-events.md), and [jump transport](020-single-jump-transport.md).

## Decision

The target's host inspection first probes its current owned, direct bastion.
If that key is unknown or changed, return the bastion fingerprint with
`hop: bastion` and `jump_connection_id`. Otherwise authenticate to the verified
bastion and inspect the target, returning `hop: target` without that ID.
Inspection never authenticates to the host whose fingerprint it returns.

Approval/reset requests echo the optional bastion ID along with endpoint and
fingerprint, using the target's existing API path. Resolve it only through the
current owned jump reference. Reject replaced/removed references and stale host
identities. Recheck the route reference and inspected endpoint in the final short
trust transaction. Approval still re-probes; reset still removes only the exact
previous trusted key and does not approve its replacement.

After bastion approval, the modal re-inspects the target route. Only verified
target trust enables Open terminal. Reject closes without changing trust. A
changed key on either hop requires explicit reset and separate approval. A
bastion change after inspection remains blocked by live handshake verification.

Safe HTTP errors include an optional `hop` (bastion or target); the browser labels
it. Terminal setup failures carry the same label in their existing status message.
Forwarding failure is attributed to the target reachability operation, without
claiming whether bastion policy, routing, or the target caused it. Timeouts retain
the active operation's hop. Never expose raw SSH or storage errors.

## Audit and lifetime

Resolve both owned destinations before audit start, and dial those recorded
snapshots. Revalidate bastion ownership/direct eligibility after audit persistence;
credential ownership and host trust remain checked at each handshake. Later
configuration edits cannot rewrite an attempt's history.

Migration `20261004000300` adds nullable `jump_snapshot` JSON with only the bastion
ID, name, host, port, and username, plus nullable `failure_hop`. Start/failure/end
retain the same bastion snapshot. A failure stores its first observed code and hop
together. Post-setup transport/I/O failures leave the hop null when the chain does
not reveal the failing component. Direct attempts and older records have no jump
snapshot. A jump lookup failure records no fabricated bastion details.

Existing close-before-audit-completion behavior releases the whole transport.
Terminal stdin writes serialize per terminal because the SSH channel writer is
not safe concurrently. The existing I/O timeout covers waiting writers too;
registry invalidation can still close the transport without waiting for that lock.

Rollback preserves existing audit rows but removes the new details. Reapplying
leaves old rows null rather than inventing history. Apply migrations explicitly
before restarting the application.

## Verification

Controlled SSH and rendered UI tests cover sequential approval, changed bastion
reset, stale-reference rejection, independent hop trust/authentication failures,
forwarding rejection, safe messages, cleanup, snapshot retention, and migration
constraints/rollback. Docker/browser shells remain a separate integration check.
