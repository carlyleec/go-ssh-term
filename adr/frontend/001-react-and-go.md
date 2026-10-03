# Serve a React application from Go

Status: Accepted

## Context

The project needs a small React interface and a self-contained demo deployment with straightforward browser authentication.

## Decision

Use React with TanStack Router, Tailwind CSS, and daisyUI. Build the frontend during the application image build and serve its compiled assets from Go on the same origin as the API. Use `/`, `/login`, and `/connections` as the primary routes.

## Consequences

Frontend development requires a build toolchain, but the runnable demo does not require a separate frontend server. Go must support direct SPA navigation without returning frontend HTML for missing API paths or assets. Styling uses daisyUI components and Tailwind utilities.
