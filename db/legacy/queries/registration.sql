-- name: CreateAccount :exec
INSERT INTO accounts (id, display_name, rp_id, webauthn_user_handle)
VALUES ($1, $2, $3, $4);

-- name: CreatePasskeyCredential :exec
INSERT INTO passkey_credentials (
    account_id, rp_id, credential_id, public_key, attestation_type,
    attestation_format, transports, flags, aaguid, sign_count,
    clone_warning, attachment, attestation, extensions
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);
