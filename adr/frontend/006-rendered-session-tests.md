# Test session UI with real routes and mocked HTTP

Status: Accepted

## Context

Helper tests cover individual requests and route guards but cannot verify that
React effects, Query observers, mutation callbacks, and navigation work together.
Session expiry also needs repeatable tests without waiting for real timeouts.

## Decision

Keep Bun as the test runner. Use React Testing Library and Happy DOM to render
the generated route tree under StrictMode, with a fresh QueryClient and memory
history per test. Mock HTTP responses at fetch; keep route components, auth
helpers, Query behavior, and Router behavior real.

Use a standalone Sinon fake clock through Query's timeout provider. Advance Query
timers deterministically while React, Router, and DOM waits retain real timers.
Dispatch browser visibility and network events to exercise Query revalidation.
These tests simulate the DOM; they do not establish real-browser timer throttling
or native passkey compatibility.

Run helper and UI suites in separate Bun processes so module mocks cannot leak
into rendered-route tests. Load Happy DOM before UI imports. Type-check both test
and application code. Include both suites in make check, with make frontend-test
available independently. Pin all test dependencies in package.json and bun.lock.

## Consequences

Session behavior has repeatable UI regression coverage without adding a second
test runner or changing production components to expose test-only entry points.
Real passkey, cookie, proxy, and serving-mode checks still require the browser
walkthrough specified in the account-access slice.
