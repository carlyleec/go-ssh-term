# Use Huma typed handlers and generate frontend API contracts

Status: Accepted; implementation is tracked in S3.5.3.

## Context

The API needs operation definitions and Go request/response types that produce
a usable OpenAPI contract. Typed handlers suit the project's preferred style
and avoid maintaining a separate documentation-only route registry.

## Decision

Adopt Huma v2 typed handlers for the HTTP API, using its humago adapter with
the existing net/http ServeMux. Register operations with explicit input/output
models and stable operation IDs. Keep feature logic and persistence in their
existing Go slices.

Migrate current-user first to establish the session integration and export
pattern, then migrate the remaining auth and SSH-key operations. Preserve URLs,
response envelopes/statuses, safe errors, cookie/session semantics, exact-origin
checks, and ownership guarantees. Configure Huma serialization, validation, and
error handling to match these contracts instead of silently adopting different
defaults. Preserve the bounded streaming multipart policy in [010](010-ssh-key-management-api.md);
use custom body handling where required by that policy.

Separate operation registration from runtime dependency initialization. A Go
export command builds the same operation definitions and writes OpenAPI JSON
without starting the server, opening SQLite, or loading encryption material.
Export must not execute handlers.

Chain two stages with contract:gen:
- Go operation definitions and models produce committed OpenAPI JSON.
- api:gen consumes that JSON to produce committed schema.gen.ts and zod.gen.ts
  under frontend/src/api/generated/. Use openapi-typescript with rootTypes.
  Generate Zod schemas for request bodies and responses, including referenced
  components; reject unsupported constructs rather than weakening schemas.

Generated artifacts carry do-not-edit banners and are excluded from Biome.
Form schemas stay handwritten in their consuming files. A debounced, serialized
development watcher runs the pipeline when Go contract inputs change. CI
regenerates all artifacts and rejects drift. Generation failures are visible
and leave the last valid outputs intact.

## Consequences

API handlers require migration, with regression checks for middleware ordering,
validation responses, and upload limits. Huma owns typed operation registration
and OpenAPI generation; the frontend client and handwritten query groups remain.
No generated client SDK or replacement database/auth library is required.

References: [Huma operations](https://huma.rocks/features/operations/),
[ServeMux adapter](https://huma.rocks/features/bring-your-own-router/),
[OpenAPI configuration](https://huma.rocks/features/openapi-generation/).
