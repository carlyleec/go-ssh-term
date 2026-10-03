-- migrate:up
CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    display_name TEXT NOT NULL CHECK (btrim(display_name) <> ''),
    rp_id TEXT NOT NULL CHECK (rp_id <> ''),
    webauthn_user_handle BYTEA NOT NULL CHECK (octet_length(webauthn_user_handle) BETWEEN 1 AND 64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (rp_id, webauthn_user_handle),
    UNIQUE (id, rp_id)
);

CREATE TABLE passkey_credentials (
    account_id UUID PRIMARY KEY,
    rp_id TEXT NOT NULL,
    credential_id BYTEA NOT NULL CHECK (octet_length(credential_id) > 0),
    public_key BYTEA NOT NULL CHECK (octet_length(public_key) > 0),
    attestation_type TEXT NOT NULL,
    attestation_format TEXT NOT NULL,
    transports TEXT[] NOT NULL DEFAULT '{}',
    flags SMALLINT NOT NULL CHECK (flags BETWEEN 0 AND 255),
    aaguid BYTEA NOT NULL CHECK (octet_length(aaguid) = 16),
    sign_count BIGINT NOT NULL DEFAULT 0 CHECK (sign_count BETWEEN 0 AND 4294967295),
    clone_warning BOOLEAN NOT NULL DEFAULT FALSE,
    attachment TEXT NOT NULL DEFAULT '',
    attestation JSONB NOT NULL CHECK (jsonb_typeof(attestation) = 'object'),
    extensions JSONB NOT NULL CHECK (jsonb_typeof(extensions) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ,
    UNIQUE (rp_id, credential_id),
    FOREIGN KEY (account_id, rp_id) REFERENCES accounts (id, rp_id) ON DELETE CASCADE
);

-- SCS owns the encoded session data and its storage queries.
CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    data BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- migrate:down
DROP TABLE sessions;
DROP TABLE passkey_credentials;
DROP TABLE accounts;
