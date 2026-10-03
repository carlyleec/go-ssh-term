-- migrate:up
CREATE TABLE accounts (
    id TEXT PRIMARY KEY NOT NULL CHECK (
        length(id) = 36 AND substr(id, 9, 1) = '-' AND substr(id, 14, 1) = '-'
        AND substr(id, 19, 1) = '-' AND substr(id, 24, 1) = '-'
        AND length(replace(id, '-', '')) = 32 AND replace(id, '-', '') NOT GLOB '*[^0-9a-f]*'
    ),
    display_name TEXT NOT NULL CHECK (trim(display_name) <> ''),
    rp_id TEXT NOT NULL CHECK (rp_id <> ''),
    webauthn_user_handle BLOB NOT NULL CHECK (length(webauthn_user_handle) BETWEEN 1 AND 64),
    -- UTC Unix nanoseconds, supplied by the application to retain full precision.
    created_at INTEGER NOT NULL,
    UNIQUE (rp_id, webauthn_user_handle),
    UNIQUE (id, rp_id)
) STRICT;

CREATE TABLE passkey_credentials (
    account_id TEXT PRIMARY KEY NOT NULL,
    rp_id TEXT NOT NULL,
    credential_id BLOB NOT NULL CHECK (length(credential_id) > 0),
    public_key BLOB NOT NULL CHECK (length(public_key) > 0),
    attestation_type TEXT NOT NULL,
    attestation_format TEXT NOT NULL,
    transports TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(transports) AND json_type(transports) = 'array'),
    flags INTEGER NOT NULL CHECK (flags BETWEEN 0 AND 255),
    aaguid BLOB NOT NULL CHECK (length(aaguid) = 16),
    sign_count INTEGER NOT NULL DEFAULT 0 CHECK (sign_count BETWEEN 0 AND 4294967295),
    clone_warning INTEGER NOT NULL DEFAULT 0 CHECK (clone_warning IN (0, 1)),
    attachment TEXT NOT NULL DEFAULT '',
    attestation TEXT NOT NULL CHECK (json_valid(attestation) AND json_type(attestation) = 'object'),
    extensions TEXT NOT NULL CHECK (json_valid(extensions) AND json_type(extensions) = 'object'),
    created_at INTEGER NOT NULL,
    last_used_at INTEGER,
    UNIQUE (rp_id, credential_id),
    FOREIGN KEY (account_id, rp_id) REFERENCES accounts (id, rp_id) ON DELETE CASCADE
) STRICT;

-- SCS owns the encoded data; expiry uses UTC Unix nanoseconds.
CREATE TABLE sessions (
    token TEXT PRIMARY KEY NOT NULL,
    data BLOB NOT NULL,
    expiry INTEGER NOT NULL
) STRICT;
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- migrate:down
DROP TABLE sessions;
DROP TABLE passkey_credentials;
DROP TABLE accounts;
