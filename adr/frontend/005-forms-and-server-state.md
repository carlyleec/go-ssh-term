# Use TanStack Query and Form for account access and workspace data

Status: Accepted

## Context

Account access introduces asynchronous server operations and form validation.
Later workspace slices add connection and key forms and server-owned lists.
These responsibilities need consistent ownership alongside TanStack Router.

## Decision

Use TanStack Query for server queries and mutations, TanStack Form for field
values, validation, and submission, and the existing TanStack Router for navigation.
Mount one QueryClientProvider at the React entry point. Use ordinary fetch for
same-origin API requests, with multipart/form-data for file uploads and JSON
elsewhere; HTTP-only cookies remain the login-session authority.
Current-user queries and route protection are implemented with the current-user
endpoint, not inferred from an earlier successful login mutation.

Use @simplewebauthn/browser for explicit passkey registration and authentication
prompts and WebAuthn JSON conversion. Go continues verifying credentials. Unwrap
the Go options response's publicKey field before invoking the browser helper.

Treat each begin/prompt/finish sequence as one mutation. Await mutateAsync from
the registration form submission. Disable competing actions and guard against
simultaneous attempts. Disable automatic retries and offline queuing for these
interactive operations; a manual retry obtains a new challenge. Preserve safe
server errors, particularly the instruction to sign in after an account was saved
but its session could not be started. Explain cancellation without asserting that
the browser distinguishes cancellation from timeout or denied permission.

Keep page components in route files and extract focused auth helpers. Use existing
Tailwind and daisyUI controls; do not introduce a general form-component framework
for the initial display-name field. Pin dependencies and commit the Bun lockfile.

## Consequences

Form and server-operation state have separate owners without duplicating field
values in React state. Later forms can reuse the same libraries. Browser passkey
compatibility still requires verification through both supported serving modes.
