# Separate workspace navigation from management drawers

Status: Accepted; replaces the route and modal choices in [002](002-terminal-workspace.md), extends [001](001-react-and-go.md), and updates terminal ownership in [008](008-first-browser-terminal.md).

## Decision

Use navbar links for Workspace (`/workspace`), Connections (`/connections`), and
SSH Keys (`/keys`). Successful sign-in and authenticated visits to `/` open Workspace. Keep page components in
their file-based route files and page-specific controls in their `-components`
directories. Reserve future tabs for terminal sessions.

Connections owns saved configurations and SSH config import. SSH Keys owns key
metadata and deletion. Upload, connection add/edit, import, connection selection,
and host verification use right-side drawers instead of centered modals. Import
uses a wider panel; all drawers fit narrow screens. Selection advances to host
verification without stacking panels. Keep native modal dialog semantics beneath
the drawer styling for focus containment, Escape, and focus restoration. Retain
explicit approval, confirmation, and pending-operation dismissal guards.

The authenticated layout owns a terminal workspace provider. It keeps the single
terminal mounted but hidden while viewing management routes, so output continues
and returning does not create a replacement shell. ResizeObserver refits the
terminal when its container becomes visible. Navigation cancels pending
reconnect presentation and closes host verification. Leaving the authenticated
layout, logout, and expiry dispose the terminal. Session checks and server-side
ownership remain authoritative.

## Consequences

Management no longer competes with terminal controls. Terminal state outlives
individual protected pages, but not the signed-in layout. Multiple terminal
tabs, aggregate limits, and automatic restoration remain future work.
