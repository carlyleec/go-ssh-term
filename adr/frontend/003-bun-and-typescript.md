# Use Bun and TypeScript for the frontend toolchain

Status: Accepted; routing choice superseded by [004](004-file-based-routing.md).

## Context

The frontend needs reproducible installs and builds in Docker alongside Go,
while host editors need local dependencies and compatible tools.

## Decision

Use TypeScript for React and Bun as the package manager and runtime for Vite and
TypeScript commands. Pin Bun in the Docker image and package manifest, use exact
direct dependency versions, and keep `bun.lock` in source control. Development
startup installs with a frozen lockfile.

Use Biome for frontend formatting, recommended lint rules, and import organization.
Enable Tailwind directive parsing and exclude generated artifacts explicitly.
Keep TypeScript checking separate from Biome checks.

The original choice of code-defined routes is superseded by
[file-based routing](004-file-based-routing.md). Continue using Vite for frontend
builds and hot updates as specified in the development workflow.

## Consequences

Dependency changes update both the manifest and Bun lockfile. Keep separate host
and Docker dependency installations from the same lockfile: the host editor reads
`frontend/node_modules`, while a Docker volume holds Linux packages. Pin matching
host Go and Bun versions in `.tool-versions` and install editor dependencies with
`make setup`. Download caches and container build output remain in Docker volumes.

Builds must be checked under the pinned Bun runtime. Route generation is governed
by the replacement routing decision.
