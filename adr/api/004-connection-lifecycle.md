# Separate saved connections from live sessions

Status: Accepted

## Context

A user may open multiple shells to one saved host. Browser interruptions and logout need predictable cleanup without requiring durable shell recovery.

## Decision

Persist saved configurations independently of live SSH sessions. Each live session belongs to a login session and runs in Go memory. Closing a terminal ends its SSH session; logout and login-session expiry close all sessions they own.

After browser refresh or transport interruption, clean up old connections and create fresh shells for previously active terminals while authentication remains valid. Remote exit, SSH failure, and explicit close do not trigger automatic reconnect. Server-restart recovery and multi-browser-tab coordination are outside v1.

## Consequences

Refreshing loses shell state, running commands, and prior output. Restoration references do not preserve SSH handles. Cleanup and connection creation must handle races with logout, and replacement attempts must be bounded. Editing or deleting a saved configuration does not terminate an already-running shell.
