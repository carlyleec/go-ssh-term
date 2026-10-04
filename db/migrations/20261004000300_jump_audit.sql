-- migrate:up
ALTER TABLE connection_audit_events ADD COLUMN jump_snapshot TEXT
    CHECK (jump_snapshot IS NULL OR json_valid(jump_snapshot));
ALTER TABLE connection_audit_events ADD COLUMN failure_hop TEXT
    CHECK (failure_hop IS NULL OR (event_type = 'failure' AND failure_hop IN ('bastion', 'target')));

-- migrate:down
ALTER TABLE connection_audit_events DROP COLUMN failure_hop;
ALTER TABLE connection_audit_events DROP COLUMN jump_snapshot;
