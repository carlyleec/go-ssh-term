# Use xterm.js in a tabbed workspace

Status: Accepted; route and modal decisions superseded by [009](009-workspace-navigation-and-drawers.md).

## Context

Users need several concurrent shells and simple configuration management without a separate detail page for every session.

## Decision

Use xterm.js for terminal emulation inside a React workspace at `/connections`. React owns connection controls and tab state; xterm.js owns terminal rendering and input. Use modals for key management, adding or editing connections, importing config, and choosing a connection to open.

Display one terminal tab at a time while other sessions remain connected. Permit multiple sessions to the same saved host. Leave split panes and multi-browser-tab coordination outside v1.

## Consequences

The app avoids implementing terminal emulation. Each terminal needs isolated input, output, resize handling, and disposal. Closing a terminal tab closes its SSH session. No `/connections/:id` detail route is required for v1.
