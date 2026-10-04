# Provide an isolated OpenSSH lab with Compose

Status: Accepted; the database service is superseded by [SQLite storage](../api/008-sqlite-storage.md). SSH lab topology remains in effect.

## Context

An evaluator should be able to exercise real SSH and bastion routing without providing external servers.

## Decision

Run the application, one Ubuntu OpenSSH bastion, and two Ubuntu OpenSSH private targets with Docker Compose. After Slice 2.5, SQLite runs inside Go with a shared persistent directory volume in both serving modes; no database service is required. Place the gateway and bastion on a gateway-facing network and the bastion and targets on a private network. Keep the gateway off the private network so target access requires the bastion.

Supply demo credentials, a matching sample SSH config, and a readable `/host-info.txt` on each host. Persist application data and encryption material in separate volumes. The deployment is local-only; configured external destinations may still be used if reachable.

Use Compose service names `bastion`, `target-1`, and `target-2`. Targets join only
`lab-private`, an internal Docker network with no published SSH ports. The
bastion also joins `lab-gateway`; the app joins only the gateway-facing and
default networks. Reuse the Ubuntu SSH image with a build-time host-info label
for each target. All three authorize the demo key for the `demo` user.

Persist each host's SSH host keys in its own dedicated named volume shared by demo and development. Generate missing keys at startup and keep SSH configuration in the image. Ordinary container replacement preserves host identity; deleting the volume replaces that identity.

## Consequences

The demo exercises real shells and SSH forwarding rather than a fake shell. Docker is required, and network isolation must be verified in the lab. Users can reset the environment by deleting volumes, but that destroys persisted data. Public deployment is outside v1.
