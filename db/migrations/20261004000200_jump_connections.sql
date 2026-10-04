-- migrate:up
ALTER TABLE saved_connections ADD COLUMN jump_connection_id TEXT
    REFERENCES saved_connections (id) ON DELETE NO ACTION;
CREATE INDEX saved_connections_jump_connection_id_idx ON saved_connections (jump_connection_id);

-- Validate in the write statement so concurrent edits cannot form a chain.
CREATE TRIGGER saved_connections_jump_insert
BEFORE INSERT ON saved_connections
BEGIN
    SELECT RAISE(ABORT, 'invalid_jump_connection')
    WHERE (NEW.jump_connection_id IS NOT NULL AND (
        NEW.jump_connection_id = NEW.id OR NOT EXISTS (
            SELECT 1 FROM saved_connections
            WHERE id = NEW.jump_connection_id AND account_id = NEW.account_id
                AND jump_connection_id IS NULL
        ) OR EXISTS (
            SELECT 1 FROM saved_connections WHERE jump_connection_id = NEW.id
        )
    )) OR EXISTS (
        SELECT 1 FROM saved_connections
        WHERE jump_connection_id = NEW.id AND account_id <> NEW.account_id
    );
END;

CREATE TRIGGER saved_connections_jump_update
BEFORE UPDATE ON saved_connections
BEGIN
    SELECT RAISE(ABORT, 'invalid_jump_connection')
    WHERE (NEW.jump_connection_id IS NOT NULL AND (
        NEW.jump_connection_id = NEW.id OR NOT EXISTS (
            SELECT 1 FROM saved_connections
            WHERE id = NEW.jump_connection_id AND account_id = NEW.account_id
                AND jump_connection_id IS NULL
        ) OR EXISTS (
            SELECT 1 FROM saved_connections WHERE jump_connection_id = OLD.id
        )
    )) OR EXISTS (
        SELECT 1 FROM saved_connections
        WHERE jump_connection_id = OLD.id AND account_id <> NEW.account_id
    );
END;

-- migrate:down
DROP TRIGGER saved_connections_jump_update;
DROP TRIGGER saved_connections_jump_insert;
DROP INDEX saved_connections_jump_connection_id_idx;
ALTER TABLE saved_connections DROP COLUMN jump_connection_id;
