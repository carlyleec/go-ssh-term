-- name: CreateSavedConnection :one
INSERT INTO saved_connections (id, account_id, name, host, port, username, ssh_key_id, created_at, updated_at, jump_connection_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: ListSavedConnections :many
SELECT * FROM saved_connections WHERE account_id = ? ORDER BY created_at DESC, id ASC;

-- name: UpdateSavedConnection :one
UPDATE saved_connections SET name = sqlc.arg(name), host = sqlc.arg(host), port = sqlc.arg(port), username = sqlc.arg(username), ssh_key_id = sqlc.arg(ssh_key_id), jump_connection_id = sqlc.narg(jump_connection_id), updated_at = max(updated_at, CAST(sqlc.arg(updated_at) AS INTEGER))
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id) RETURNING *;

-- name: DeleteSavedConnection :execrows
DELETE FROM saved_connections WHERE id = ? AND account_id = ?;
