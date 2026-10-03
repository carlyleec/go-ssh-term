# Provide an isolated OpenSSH lab with Compose

Status: Accepted

## Context

An evaluator should be able to exercise real SSH and bastion routing without providing external servers.

## Decision

Run the application, Postgres, one Ubuntu OpenSSH bastion, and two Ubuntu OpenSSH private targets with Docker Compose. Place the gateway and bastion on a gateway-facing network and the bastion and targets on a private network. Keep the gateway off the private network so target access requires the bastion.

Supply demo credentials, a matching sample SSH config, and a readable `/host-info.txt` on each host. Persist application data and encryption material in separate volumes. The deployment is local-only; configured external destinations may still be used if reachable.

## Consequences

The demo exercises real shells and SSH forwarding rather than a fake shell. Docker is required, and network isolation must be verified in the lab. Users can reset the environment by deleting volumes, but that destroys persisted data. Public deployment is outside v1.
