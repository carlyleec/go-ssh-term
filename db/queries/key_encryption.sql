-- name: ListEncryptedSSHKeys :many
SELECT id, account_id, encrypted_private_key FROM ssh_keys;
