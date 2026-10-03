-- name: CreateSSHKey :exec
INSERT INTO ssh_keys (id, account_id, name, public_fingerprint, encrypted_private_key, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListSSHKeyMetadata :many
SELECT id, name, public_fingerprint, created_at FROM ssh_keys
WHERE account_id = ? ORDER BY created_at DESC, id ASC;

-- name: DeleteSSHKey :execrows
DELETE FROM ssh_keys WHERE id = ? AND account_id = ?;
