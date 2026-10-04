-- migrate:up
-- The composite parent key prevents cross-account key references.
CREATE UNIQUE INDEX ssh_keys_id_account_id_idx ON ssh_keys (id, account_id);

CREATE TABLE saved_connections (
    id TEXT NOT NULL CHECK (
        length(id) = 36 AND substr(id, 9, 1) = '-' AND substr(id, 14, 1) = '-'
        AND substr(id, 19, 1) = '-' AND substr(id, 24, 1) = '-'
        AND length(replace(id, '-', '')) = 32 AND replace(id, '-', '') NOT GLOB '*[^0-9a-f]*'
    ) PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (trim(name) <> ''),
    host TEXT NOT NULL CHECK (trim(host) <> ''),
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username TEXT NOT NULL CHECK (trim(username) <> ''),
    ssh_key_id TEXT NOT NULL,
    -- UTC Unix nanoseconds, supplied by the application.
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL CHECK (updated_at >= created_at),
    -- NO ACTION permits the account cascade to remove both connections and keys.
    FOREIGN KEY (ssh_key_id, account_id) REFERENCES ssh_keys (id, account_id) ON DELETE NO ACTION
) STRICT;
CREATE INDEX saved_connections_account_id_idx ON saved_connections (account_id);
CREATE INDEX saved_connections_ssh_key_id_account_id_idx ON saved_connections (ssh_key_id, account_id);

CREATE TABLE host_trust (
    account_id TEXT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    host TEXT NOT NULL CHECK (trim(host) <> ''),
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    -- SSH wire-format public key; derive the display fingerprint from these bytes.
    public_key BLOB NOT NULL CHECK (length(public_key) > 0),
    trusted_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, host, port)
) STRICT;

CREATE TABLE connection_audit_events (
    id TEXT NOT NULL CHECK (
        length(id) = 36 AND substr(id, 9, 1) = '-' AND substr(id, 14, 1) = '-'
        AND substr(id, 19, 1) = '-' AND substr(id, 24, 1) = '-'
        AND length(replace(id, '-', '')) = 32 AND replace(id, '-', '') NOT GLOB '*[^0-9a-f]*'
    ) PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    -- Historical identifiers, not FKs: a live shell can outlast its saved configuration.
    saved_connection_id TEXT NOT NULL CHECK (
        length(saved_connection_id) = 36 AND substr(saved_connection_id, 9, 1) = '-' AND substr(saved_connection_id, 14, 1) = '-'
        AND substr(saved_connection_id, 19, 1) = '-' AND substr(saved_connection_id, 24, 1) = '-'
        AND length(replace(saved_connection_id, '-', '')) = 32 AND replace(saved_connection_id, '-', '') NOT GLOB '*[^0-9a-f]*'
    ),
    attempt_id TEXT NOT NULL CHECK (
        length(attempt_id) = 36 AND substr(attempt_id, 9, 1) = '-' AND substr(attempt_id, 14, 1) = '-'
        AND substr(attempt_id, 19, 1) = '-' AND substr(attempt_id, 24, 1) = '-'
        AND length(replace(attempt_id, '-', '')) = 32 AND replace(attempt_id, '-', '') NOT GLOB '*[^0-9a-f]*'
    ),
    connection_name TEXT NOT NULL CHECK (trim(connection_name) <> ''),
    host TEXT NOT NULL CHECK (trim(host) <> ''),
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username TEXT NOT NULL CHECK (trim(username) <> ''),
    event_type TEXT NOT NULL CHECK (event_type IN ('start', 'end', 'failure')),
    -- Application-owned safe codes only; never store raw SSH errors or terminal data.
    failure_code TEXT CHECK (
        length(failure_code) BETWEEN 1 AND 64 AND failure_code NOT GLOB '*[^a-z0-9_]*'
    ),
    occurred_at INTEGER NOT NULL,
    CHECK ((event_type = 'failure' AND failure_code IS NOT NULL)
        OR (event_type <> 'failure' AND failure_code IS NULL))
) STRICT;
CREATE INDEX connection_audit_events_account_time_idx ON connection_audit_events (account_id, occurred_at);
CREATE INDEX connection_audit_events_account_attempt_idx ON connection_audit_events (account_id, attempt_id);

-- migrate:down
DROP TABLE connection_audit_events;
DROP TABLE host_trust;
DROP TABLE saved_connections;
DROP INDEX ssh_keys_id_account_id_idx;
