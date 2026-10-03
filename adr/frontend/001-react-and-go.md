# Serve a React application from Go

Status: Accepted

## Context

The project needs a small React interface and a self-contained demo deployment with straightforward browser authentication.

## Decision

Use React with TanStack Router, Tailwind CSS, and daisyUI. Build the frontend during the application image build and serve its compiled assets from Go on the same origin as the API. Use `/`, `/login`, and `/connections` as the primary routes.

Serve compiled files from `frontend/dist` on disk. During development, share the
frontend build volume read-only with Go; the application image will contain the
compiled files. Fall back to `index.html` for extensionless page URLs, reserving
`/api` and `/assets` and returning 404 for missing file URLs.

## Consequences

Frontend development requires a build toolchain, but the runnable demo does not require a separate frontend server. Go must support direct SPA navigation without returning frontend HTML for missing API paths or assets. Styling uses daisyUI components and Tailwind utilities.
