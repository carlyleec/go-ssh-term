-- name: LockLoginCredential :one
SELECT sqlc.embed(accounts), sqlc.embed(passkey_credentials)
FROM accounts
JOIN passkey_credentials ON passkey_credentials.account_id = accounts.id AND passkey_credentials.rp_id = accounts.rp_id
WHERE accounts.rp_id = $1 AND passkey_credentials.credential_id = $2 AND accounts.webauthn_user_handle = $3
FOR UPDATE OF passkey_credentials;

-- name: UpdateLoginCredential :exec
UPDATE passkey_credentials
SET sign_count = $3, clone_warning = $4, flags = $5, last_used_at = CURRENT_TIMESTAMP
WHERE account_id = $1 AND rp_id = $2;
