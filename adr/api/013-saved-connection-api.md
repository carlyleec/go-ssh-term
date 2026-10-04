# Expose owner-scoped saved connection operations

Status: Accepted

## Context

Connection forms need a stable API before SSH dialing is implemented. Saved
configuration changes must not act on live shells. Follow [storage decisions](012-connection-storage.md)
and the [Huma contract pipeline](011-huma-and-generated-contracts.md).

## Decision

Protect all operations with the existing verified-account middleware and require
the configured Origin for mutations. Ownership comes only from request context.

- `POST /api/connections` creates a configuration and returns 201 with
  `{"connection": ...}`. `PUT /api/connections/{id}` replaces its editable fields
  and returns the same envelope with 200, preserving identity and creation time.
- `GET /api/connections` returns `{"connections": [...]}`, newest first, with ID
  as the tie-breaker. Empty lists are arrays. `DELETE /api/connections/{id}`
  returns an empty 204. Missing, malformed, and unowned IDs return 404 for valid
  update requests and deletes.
- Create/update require all five fields: `name`, `host`, `port`, `username`, and
  `ssh_key_id`. Accept only JSON, reject unknown fields, and bound bodies to 4 KiB.
  Trim name/host/username. Names contain 1–64 Unicode characters without controls;
  usernames contain 1–64 ASCII letters, digits, underscores, dots, or hyphens,
  starting with a letter, digit, or underscore. Ports are integers from 1 to 65535.
- Hosts are ASCII DNS names (including single-label lab names) or unbracketed IP
  addresses. Reject URLs, embedded ports, IPv6 zones, invalid labels, and numeric
  names that are not valid IP addresses. Lowercase DNS names and remove a terminal
  dot; canonicalize IP spellings and unmap IPv4-mapped IPv6. Do not resolve hosts
  during CRUD. Later SSH/trust code must reuse this normalization. International
  names must use their ASCII/Punycode representation.
- Responses contain ID, the five fields, and UTC creation/update timestamps, never
  key material or account IDs. Each write uses a single SQL statement; database
  foreign keys arbitrate races with key deletion and enforce key ownership.
  Missing/unowned keys yield the same safe 400. Deleting an owned referenced SSH
  key yields 409; another account's key still yields 404.
- Validation failures use safe `{"error": ...}` envelopes with 400; oversized
  bodies use 413, unsupported content types 415, and storage failures 503.
  Preserve no-store and existing session behavior. Generated contracts describe
  wire limits; server validation additionally applies normalization and ownership.

## Consequences

Edits and deletes affect saved data only. SSH dialing, host approval, terminal
handles, and frontend connection controls remain separate tasks. Host trust is
not silently created, updated, or removed by these operations.
