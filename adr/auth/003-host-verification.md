# Require explicit SSH host trust

Status: Accepted

## Context

Authenticating a user to a target does not establish that the target is the intended SSH server.

## Decision

Show an unfamiliar host's fingerprint and require approval before authentication proceeds. Persist trust per user and host endpoint. Block changed host keys and require an explicit trust-reset procedure. Apply verification independently to bastion and target hosts.

## Consequences

The first connection requires an extra user decision. Trust on first use depends on the user's initial approval; it does not independently establish host identity. Host replacement requires deliberate trust updates. [SSH dialing and trust decisions](013-ssh-dialing-and-trust-decisions.md) defines the explicit reset interface, with no silent bypass.
