# Use Huma typed handlers and generate frontend API contracts

Status: Accepted; implementation is tracked in S3.5.3. The ServeMux/humago
choice is superseded by [023](023-explicit-grouped-routing.md); the typed-handler
and generated-contract decisions remain in force.

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

Passkey finish middleware consumes the challenge before Huma reads or decodes
its typed body. go-webauthn remains responsible for credential verification.
Scope Huma's error customization to ceremony operations so decoding and body-limit
failures retain safe 400 envelopes without reflecting credential data. Commit
all ceremony sessions explicitly before success, including registration begin;
store failures return 503 without a success body or new cookie. Schema aliases
account for upstream base64url types and optional browser attestation fields.

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
Form schemas stay handwritten in their consuming files. Generation tools have a
separate locked package to retain TypeScript 5 compiler APIs for openapi-typescript
while the application uses TypeScript 7. A debounced, serialized
development watcher runs the pipeline when Go contract inputs change. Generation
and verification run locally; this demo has no CI. Generation failures are visible
and leave the last valid outputs intact.

## Consequences

API handlers require migration, with regression checks for middleware ordering,
validation responses, and upload limits. Huma owns typed operation registration
and OpenAPI generation; the frontend client and handwritten query groups remain.
No generated client SDK or replacement database/auth library is required.

References: [Huma operations](https://huma.rocks/features/operations/),
[ServeMux adapter](https://huma.rocks/features/bring-your-own-router/),
[OpenAPI configuration](https://huma.rocks/features/openapi-generation/).
