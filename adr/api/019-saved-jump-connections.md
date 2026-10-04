# Enforce one owned jump in saved connections

Status: Accepted; extends [connection storage](012-connection-storage.md) and
implements the saved-reference portion of [one SSH jump](005-ssh-configuration.md).

## Decision

Add nullable `jump_connection_id` to `saved_connections`, referencing another
saved connection with `ON DELETE NO ACTION` and an index for dependent lookups.
Existing rows remain direct. Each jump must belong to the same account and must
itself be direct. Reject self references and any attempt to give a referenced
bastion its own jump. These rules also exclude cycles of any length.

Enforce ownership and graph rules with SQLite insert/update triggers in the
write statement. This closes races between concurrent edits without a separate
read/validate/write sequence. Check incoming dependents on owner changes as well.
The self foreign key prevents dangling references while allowing account deletion
to cascade through all owned configurations. Referenced bastion deletion returns
409; callers must first remove or replace its incoming jump references.

Expose `jump_connection_id` as an optional canonical UUID in create/update input
and saved-connection output. Omitted or null input means no jump, including on
PUT; an empty string is invalid. Omit the response field for direct connections.
Missing, foreign, chained, and self references return a safe 400 message without
revealing foreign configuration details. Invalid key references retain their
existing error behavior. Unrelated frontend edits preserve the loaded jump ID.

## Migration and staged rollout

Migration `20261004000200` adds the column, index, and triggers without rebuilding
existing connection data. Rollback drops only this extension; saved configurations
remain but lose jump routing. Reapplication leaves those configurations direct.
Use the existing explicit migration workflow before restarting the application.

The [single-jump transport](020-single-jump-transport.md) forwards target probes
and dialing through the selected bastion, with no direct fallback. Guided per-hop
trust flow and audit snapshots for both hops remain subsequent work; the wire
terminal protocol is unchanged.

## Connection forms

The route-owned create/edit form uses the account-scoped connection query for
Jump through choices. Offer only other direct destinations; a referenced bastion
can only remain direct. Initialize existing values and send null for explicit
direct selection. Keep unavailable selected IDs visible with an error instead
of silently dropping them. Block submission when the list is unavailable or the
selected jump is invalid; preserve values across retries and server validation
errors. The server remains authoritative if the configuration changes during save.

Saved rows display the selected jump name. A deletion conflict retains the row
and its error so the user can cancel, change dependent references, and retry.

## Verification

HTTP and file-backed SQLite tests cover round trips, replacement/removal, foreign
and missing references, self/cyclic/chained references, edits to referenced
bastions, concurrent edits, deletion protection, account cascades, rollback and
reapplication, and rejection of direct fallback. Rendered frontend tests cover creating/removing/preserving jumps, filtered
choices, stale selections, list/save errors, pending saves, and deletion-conflict
recovery. Pinned dbmate verifies
up/down/up on a disposable database.
