-- name: InsertConnectionAuditEvent :exec
INSERT INTO connection_audit_events (
    id, account_id, saved_connection_id, attempt_id, connection_name,
    host, port, username, event_type, failure_code, occurred_at, jump_snapshot, failure_hop
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
