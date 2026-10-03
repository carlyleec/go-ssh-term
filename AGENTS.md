# Working in this repository

This is a small Go and React browser SSH gateway built as a learning and portfolio project. Keep changes focused, readable, and organized by vertical slice. The user prefers to write most application code themselves: provide guidance or review unless they ask for implementation. A request to implement a task authorizes that task, not the rest of the plan.

## Use the project documents

- [PRD.md](PRD.md) defines product scope, behavior, acceptance criteria, and exclusions. Read the relevant slice and any applicable cross-cutting requirements.
- [DEV_PLAN.md](DEV_PLAN.md) lists implementation tasks in slice order. Locate the requested task by ID, such as `S1.3`, and check its prerequisites against the actual code before starting.
- [adr/README.md](adr/README.md) indexes architecture decisions. Read only records relevant to the task, following related decisions when necessary. Accepted decisions describe the intended architecture, not proof of implementation.

Start with the requested task and relevant documents; avoid rereading every document for each small change. Inspect existing code and preserve unrelated work. If documents conflict materially, surface the discrepancy instead of silently choosing a new architecture. Follow explicit user direction and update affected documents when scope or decisions change.

## Track work

- Reference task IDs in progress updates and completion summaries.
- Keep IDs stable. Add new tasks using the next unused number within their slice; do not renumber existing tasks or reuse removed IDs.
- Check off a task only when its work and relevant verification are complete. Leave partial or blocked tasks unchecked and report what remains.
- Run focused checks appropriate to the change and report results and limitations. Do not claim a feature works just because its task is listed or its architecture is accepted.
- Record significant new architectural decisions in the appropriate ADR directory. When replacing a decision, link the replacement and mark the old record superseded. Keep records short and free of conversation history.
- Keep PRD requirements, plan tasks, and ADR decisions consistent without duplicating the same detail across all three.
