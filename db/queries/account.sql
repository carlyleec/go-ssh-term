-- name: GetSessionAccount :one
SELECT id, display_name FROM accounts WHERE id = $1 AND rp_id = $2;
