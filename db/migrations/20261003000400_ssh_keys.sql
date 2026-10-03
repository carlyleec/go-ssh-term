-- migrate:up
CREATE TABLE ssh_keys (
    id TEXT PRIMARY KEY NOT NULL CHECK (
        length(id) = 36 AND substr(id, 9, 1) = '-' AND substr(id, 14, 1) = '-'
        AND substr(id, 19, 1) = '-' AND substr(id, 24, 1) = '-'
        AND length(replace(id, '-', '')) = 32 AND replace(id, '-', '') NOT GLOB '*[^0-9a-f]*'
    ),
    account_id TEXT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (trim(name) <> ''),
    public_fingerprint TEXT NOT NULL CHECK (trim(public_fingerprint) <> ''),
    -- Opaque encrypted payload; its encoding is owned by the encryption code.
    encrypted_private_key BLOB NOT NULL CHECK (length(encrypted_private_key) > 0),
    -- UTC Unix nanoseconds, supplied by the application.
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX ssh_keys_account_id_idx ON ssh_keys (account_id);

-- migrate:down
DROP TABLE ssh_keys;
