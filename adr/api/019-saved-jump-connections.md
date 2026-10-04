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

Until SSH forwarding is implemented, the transport rejects any attempt to dial
or probe a configuration carrying a jump. It must never silently connect directly
to the target. The selector, forwarding, per-hop trust flow, and audit snapshots
for both hops remain subsequent work; the wire terminal protocol is unchanged.

## Verification

HTTP and file-backed SQLite tests cover round trips, replacement/removal, foreign
and missing references, self/cyclic/chained references, edits to referenced
bastions, concurrent edits, deletion protection, account cascades, rollback and
reapplication, and rejection of direct fallback. A rendered frontend test checks
that editing other fields preserves the existing jump. Pinned dbmate verifies
up/down/up on a disposable database.
