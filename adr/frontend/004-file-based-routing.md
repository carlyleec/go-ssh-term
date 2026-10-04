# Use file-based TanStack Router routes

Status: Accepted; supersedes the routing choice in [003](003-bun-and-typescript.md).

## Context

The frontend needs an explicit, discoverable mapping between URLs and page files
without manually maintaining a route tree.

## Decision

Define routes under `frontend/src/routes` using TanStack Router's file conventions.
Use directory-based routes and colocated components as specified in [007](007-frontend-organization.md).
Keep page components in their route files and the shared layout on the root route.
Use the Vite router plugin during development and the router CLI before type
checking to generate `src/routetree.gen.ts`. Share generator settings through
`tsr.config.json` and preserve lowercase filenames.

Keep the generated route tree in source control so editors can resolve route
types immediately after dependency installation. Regenerate it when routes change;
do not edit it manually or run Biome on it.

## Consequences

Adding a route requires a route file rather than manual tree registration. Both
host and container installations need the generator dependencies. The remaining
Bun, TypeScript, Biome, and dependency isolation decisions in 003 still apply.
