# Support one SSH jump and a limited config importer

Status: Accepted

## Context

The lab needs bastion access, and users should be able to import recognizable SSH configuration without building a complete OpenSSH configuration engine.

## Decision

Allow direct connections or one jump through another saved connection owned by the same user. Support importing `Host`, `HostName`, `User`, `Port`, `IdentityFile`, and one `ProxyJump`. Preview imports, resolve keys and jumps, and require confirmation before saving.

Treat `IdentityFile` as a mapping hint for an uploaded key. Never execute imported commands or read referenced filesystem paths. Report unsupported syntax and directives explicitly. The exact supported syntax and transactional import contract are defined in [022](022-ssh-config-import.md).

## Consequences

Users can configure reachable hosts beyond the lab, but cannot expect full OpenSSH compatibility. Reject self references and multi-hop chains. Referenced bastion configurations cannot be deleted until their dependents are updated or removed.
