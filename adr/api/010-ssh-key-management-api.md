# Expose owner-scoped SSH key management with bounded multipart uploads

Status: Accepted

## Context

The key-management modal needs upload, list, and delete operations without
exposing private material. Uploaded files must not spill to temporary disk or
hold database transactions open while the browser sends their contents.

## Decision

Protect the `/api/keys` operations with `Access.Require` through its Huma adapter. Take ownership only
from the verified account in request context; mutations require the configured
Origin. Do not refresh sessions or issue cookies from key-management requests.

- `POST /api/keys`: accept multipart/form-data with exactly one `name` field and
  one `private_key` part. Stream parts under a 32 KiB total request limit and a
  16 KiB private-key limit. Bound raw name data to 256 bytes; trim surrounding
  whitespace and require 1–64 valid UTF-8 characters without control characters.
  Reject duplicate/unknown fields and transfer-encoded parts. Ignore filenames.
- Validate the key before encryption. Generate the record UUID server-side, bind
  encryption to that UUID and the authenticated account, then insert in one
  atomic database statement. No transaction spans upload reads or encryption.
- Return 201 with `{"key": ...}` after storage succeeds. `GET /api/keys` returns
  `{"keys": [...]}`, newest first with UUID as a stable tie-breaker. An empty list
  is `[]`. Metadata contains only `id`, `name`, `public_fingerprint`, and
  `created_at` (UTC RFC3339 timestamp with fractional precision).
- `DELETE /api/keys/{id}` scopes its single statement by record and account IDs.
  Return 204 after deletion. Malformed, missing, and another owner's IDs all
  return the same 404. Names and fingerprints may repeat.
- Invalid fields or keys return 400, oversized requests/files 413, and unsupported
  media types 415. Storage/encryption failures return safe 503 errors. Responses
  inherit no-store and session/origin protections from the access middleware.

Huma owns typed operation registration and responses. Attach the multipart
request schema after registering the upload handler: supplying it during
registration enables Huma body decoding. Only the streaming parser may read
upload bodies; regression tests enforce rejection before reads where possible.

List queries select only public metadata. There is no private-key download
endpoint, and neither plaintext nor encrypted material is serialized into API
responses. Clear the upload buffer after use and never log parser/storage errors
that might expose supplied content. Cryptographic helpers do not authorize users;
handlers and scoped queries enforce ownership.

## Consequences

The frontend submits FormData and lets the browser set the multipart boundary.
Single-row statements avoid read/delete races and shared-pool deadlocks.
Saved connections now protect referenced keys at the database level. Deleting
an owned referenced key returns a safe 409 conflict; unowned keys still return
404. See [saved connection operations](013-saved-connection-api.md).

Related: [access middleware](../auth/009-current-user-and-route-protection.md),
[key policy](../auth/002-ssh-key-storage.md),
[encryption format](../auth/012-application-encryption-material.md).
