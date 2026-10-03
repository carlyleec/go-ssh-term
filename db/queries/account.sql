-- name: GetSessionAccount :one
SELECT id, display_name FROM accounts WHERE id = ? AND rp_id = ?;
