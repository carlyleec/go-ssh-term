# Organize code by vertical slice

Status: Accepted

## Context

The project needs to remain small enough to read and implement incrementally. Features span browser interactions, HTTP handlers, and persistence.

## Decision

Organize feature code by vertical slice, keeping related behavior together. Use Go for the gateway and implement each slice through the frontend, API, and database. Share infrastructure where needed without introducing generic service or repository frameworks.

## Consequences

Each slice delivers usable behavior and focused verification. Some straightforward feature-specific code may be repeated before a shared abstraction becomes justified. Cross-cutting ownership and lifecycle rules still apply across slices.
