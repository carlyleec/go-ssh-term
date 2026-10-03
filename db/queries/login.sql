-- name: GetLoginCredential :one
SELECT sqlc.embed(accounts), sqlc.embed(passkey_credentials)
FROM accounts
JOIN passkey_credentials ON passkey_credentials.account_id = accounts.id AND passkey_credentials.rp_id = accounts.rp_id
WHERE accounts.rp_id = ? AND passkey_credentials.credential_id = ? AND accounts.webauthn_user_handle = ?;

-- name: UpdateLoginCredential :exec
UPDATE passkey_credentials
SET sign_count = ?, clone_warning = ?, flags = ?, last_used_at = ?
WHERE account_id = ? AND rp_id = ?;
