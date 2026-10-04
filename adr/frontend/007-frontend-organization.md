# Organize the frontend by routes, shared components, and API modules

Status: Accepted; supersedes the frontend folder organization implied by [API 001](../api/001-vertical-slices.md). Extends [004](004-file-based-routing.md) and [005](005-forms-and-server-state.md).

## Context

Incremental vertical slices remain useful for delivery, but separate frontend
feature folders obscure page ownership and scatter server-state definitions.

## Decision

Keep pages in directory-based route files such as
`routes/_authed/connections/index.tsx`.
Keep the root layout in `routes/__root.tsx` and the landing page in
`routes/index.tsx`. Put components used only by a route in its `-components`
directory, which TanStack Router excludes from route generation.

Put shared UI in `components`, including thin Button and Input controls using
daisyUI and native React props. Keep form-validation functions and presentation
helpers in the route or component file that uses them.

Use `routes/_authed/route.tsx` as a pathless layout for protected children.
Its beforeLoad verifies the server session before rendering, and its component
owns polling, pending/error UI, expiry cleanup, and the Outlet. The pathless
directory does not change public URLs. Login keeps its signed-in redirect.

Expose account state and logout through `hooks/use-auth.ts`, backed by the same
Query cache as the layout. After confirmed logout, the hook cancels an in-flight
account check and marks the session anonymous. The layout alone clears private
cached data and navigates to login for both logout and expiry. The
layout owns the polling observer so child hooks do not add polling intervals.
Do not add a second account store or auth provider.

Keep the API layer to three files: `api/apiClient.ts` owns fetch calls and shared
HTTP defaults, JSON/multipart handling, response parsing, and typed `ApiError`
failures. Its get/post/delete methods return parsed response bodies and preserve
cancellation. `api/endpoints.ts` exports the `ENDPOINTS` URL constants, and
`api/queries.ts` contains API types and query/mutation definitions, including the
passkey begin/prompt/finish sequence. Define operations under their domain
groups; keep routing, session lifecycle, and form validation out of this file.
Export a default
`queries` object from `api/queries.ts`, grouped by domain: currently `auth` and
`keys`, with `connections` added when its API exists. Expose query hooks for
components and query options for route guards and cache operations. Expose
mutation options directly on each operation without an extra options wrapper.
Keep UI effects such as navigation, dialog feedback, and form resets with their
consumers. Keep endpoint-specific decisions (such as treating a current-user 401
as anonymous) with queries and session consumers. Preserve existing query keys,
cancellation, and session behavior.

Use the explicitly requested `apiClient.ts` name; keep other filenames lowercase
and regenerate the route tree after route moves.

## Consequences

Shared UI, page-specific UI, and API behavior have explicit homes without
introducing a parallel pages or features hierarchy. Delivery remains organized
by vertical slices, and Go organization is unchanged.
