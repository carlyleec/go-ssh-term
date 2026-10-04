# Import a strict, reviewed SSH config subset

Status: Accepted

## Context

The lab needs a predictable importer, not an OpenSSH configuration evaluator. This refines [005](005-ssh-configuration.md) and uses the owned single-jump graph in [019](019-saved-jump-connections.md).

## Decision

Accept UTF-8 text up to 64 KiB, at most 100 Host blocks, and lines up to 4096 bytes. Accept LF or CRLF, blank lines, indentation, and full-line comments. Directives are case-insensitive and separated from one unquoted value by spaces or tabs. Alias matching and saved-name conflicts are case-sensitive.

Each `Host` has exactly one literal alias of 1–64 ASCII letters, digits, underscores, dots, or hyphens, starting with a letter, digit, or underscore. The alias becomes the saved name. Require exactly one `HostName`, `User`, and `IdentityFile` per block. `Port` defaults to 22; an explicit port is decimal 1–65535. Hostnames and usernames use saved-connection validation. `IdentityFile` is an opaque mapping hint, never a path to read or expand. Users explicitly map each selected entry to an owned uploaded key.

An optional `ProxyJump` contains one literal alias using the same syntax as `Host`. Resolve it to a selected direct entry, or an explicitly chosen existing owned direct connection with that name. Never silently substitute a deselected dependency. Reject self references, cycles, and chains. `user@host`, `host:port`, URI, comma lists, and `none` semantics are unsupported; configure endpoints and credentials in the referenced Host block instead.

Reject duplicate aliases/directives, global directives, multiple Host patterns, wildcards, negation, inheritance, Match, Include, ProxyCommand, unknown directives, equals separators, quoting, escaping, inline comments, continuations, and token/environment expansion. Report line-numbered diagnostics; any parse diagnostic blocks confirmation of that document. No commands execute and no referenced files are accessed.

Upload config text as a bounded JSON request. Preview and confirmation carry the original text plus selected aliases and explicit key/jump mappings. Preview reads only owner-scoped metadata and performs no writes. Confirmation reparses and revalidates the whole selection inside one SQLite transaction, checks current names and references, inserts direct entries before dependent entries, and commits all or none. Existing names are never overwritten. A changed or missing existing jump must be previewed again; its update timestamp accompanies the mapping. No preview state or file is persisted server-side.

## Consequences

Some valid OpenSSH files are deliberately rejected with actionable diagnostics. Names already duplicated by manual saves are conflicts, not an arbitrary resolution. A selected dependency takes precedence over existing records only when it is explicitly included in the selection. Limits bound parsing, preview size, and transaction work.
