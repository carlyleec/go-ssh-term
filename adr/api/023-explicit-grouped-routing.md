# Declare explicit grouped routes with Chi and Huma

Status: Accepted; implementation is tracked in S6.5.2–S6.5.5. Supersedes only
the ServeMux/humago choice in [011](011-huma-and-generated-contracts.md) and
the ServeMux registration choice in [014](014-terminal-websocket-protocol.md).

## Context

Startup hides HTTP methods, paths, and access rules behind feature-owned
registration methods. Runtime and OpenAPI export separately enumerate those
methods. The application should be understandable from one explicit route map
without moving feature behavior into the server entry point.

## Decision

Use Chi v5 through Huma's `humachi` adapter. Retain Huma typed inputs/outputs,
validation, serialization, and generated OpenAPI/TypeScript/Zod contracts.
Keep API traffic isolated from the frontend fallback. Register terminal
WebSockets directly with Chi, outside Huma's typed response pipeline; retain
their authentication, origin checks, protocol, and coordinated shutdown.

Put the shared HTTP route declarations in `internal/routing/routes.go`.
This composition package may import feature packages and `internal/api`;
feature packages must not depend on it. Keep API configuration in `internal/api`
and handler logic, models, and persistence in their existing vertical slices.
`run` constructs runtime dependencies and supplies them to the route map.
`cmd/openapi` invokes the same declarations with inert dependencies, without
opening storage, loading encryption material, or executing handlers. Readiness,
frontend serving, and WebSocket wiring remain explicit runtime routes and do
not become typed OpenAPI operations as part of this refactor.

Declare each HTTP method, relative path, and handler directly with `huma.Get`,
`huma.Post`, `huma.Put`, or `huma.Delete`. Use Huma groups for API prefixes and
shared operation middleware so routing and OpenAPI paths agree. Use a single
Huma API/document; do not create independent API documents per Chi group.
Chi owns raw HTTP dispatch and runtime routes. Do not apply the same prefix
at both the Chi and Huma layers.

Use receiver/action handler names such as `accounts.GetCurrent`,
`sessions.Logout`, `registration.Begin`, and `login.Finish`. Choose receivers
around actual responsibilities; do not add a type per endpoint just for naming.
Feature helpers may supply schemas or operation options, but must not hide
method/path/handler declarations behind another route-registration wrapper.

Keep access rules visible beside the declarations. A shared `/api/auth` prefix
does not imply shared authentication: current-user requires a verified account,
login/registration accept unauthenticated callers, and logout permits expired
or anonymous sessions to clear cookies. Apply common protections only to routes
that share them. Preserve middleware ordering, including consuming passkey
challenges before body parsing and authenticating uploads before reading them.

Set existing operation IDs, summaries, security declarations, responses, limits,
and custom validation options explicitly; convenience-helper defaults must not
change the contract. Preserve the post-registration multipart schema attachment
that keeps Huma from buffering private-key uploads. Migrate `humago.Unwrap`
usage in auth middleware, passkey resolvers, and uploads to the selected adapter.

## Compatibility and verification

Inspected the pinned Huma v2.39.1 source: `group.go` provides `NewGroup`,
`UseMiddleware`, and `UseModifier`; `huma.go` provides the method helpers with
`func(*huma.Operation)` options applied before registration; `adapters/humachi`
provides the Chi v5 adapter and `Unwrap`. Its module declares Chi v5.3.1.
Add an explicit compatible Chi dependency during implementation; no Huma upgrade
is required for these APIs. This source inspection does not verify the migrated
application.

Before completing the refactor, verify stable operation IDs and generated
artifacts, service-free export, API/SPA boundaries, unknown paths, unsupported
methods and Allow headers, HEAD behavior, trailing slashes, and path-parameter
decoding. Do not assume Chi and ServeMux defaults are identical. Exercise auth,
streaming upload, WebSocket upgrade, and shutdown regressions as well as the
browser workflows in the plan.

## Consequences

One route map exposes the application's HTTP surface and feeds both serving and
schema export. Handler methods and selected middleware/options need exportable
APIs, but business logic stays in feature packages. This is a routing and code
organization change; existing product behavior and security guarantees remain.

References: [Huma router adapters](https://huma.rocks/features/bring-your-own-router/),
[Huma groups](https://huma.rocks/features/groups/),
[Chi routing](https://github.com/go-chi/chi).
