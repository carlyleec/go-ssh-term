-- name: GetOwnedConnection :one
SELECT * FROM saved_connections WHERE id = ? AND account_id = ?;

-- name: GetHostTrust :one
SELECT * FROM host_trust WHERE account_id = ? AND host = ? AND port = ?;

-- name: ApproveHostTrust :exec
INSERT INTO host_trust (account_id, host, port, public_key, trusted_at)
VALUES (?, ?, ?, ?, ?) ON CONFLICT (account_id, host, port) DO NOTHING;

-- name: ResetHostTrust :execrows
DELETE FROM host_trust WHERE account_id = ? AND host = ? AND port = ? AND public_key = ?;

-- name: GetOwnedEncryptedSSHKey :one
SELECT encrypted_private_key FROM ssh_keys WHERE id = ? AND account_id = ?;
