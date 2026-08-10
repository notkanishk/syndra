-- 000034_retire_the_lldap_bridge.down.sql
--
-- This rollback restores the SHAPE and cannot restore the CONTENT, and the
-- difference matters enough to say twice. The hashes are gone. Rolling back
-- gives you three columns that every existing row violates — which is why the
-- NOT NULLs are not restored with them: reinstating those would fail on the
-- first row, and a rollback that cannot run is not a rollback.
--
-- The queue comes back empty. Anything that was in flight when 000034 ran was
-- dropped with the table, and the sync service that would have claimed it no
-- longer exists in this repository. Restoring this schema is the first step of
-- resurrecting a subsystem, not an undo.

CREATE TABLE IF NOT EXISTS provisioning_intents (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target_uid       VARCHAR(255) NOT NULL,
    action           TEXT         NOT NULL CHECK (action IN ('add', 'remove')),
    lldap_group      TEXT         NOT NULL,
    source_project   VARCHAR(255) NOT NULL,
    source_role      VARCHAR(255) NOT NULL,
    webhook_event_id VARCHAR(255),
    idempotency_key  TEXT         NOT NULL UNIQUE,
    status           TEXT         NOT NULL DEFAULT 'pending'
                                  CHECK (status IN ('pending', 'in_flight', 'completed', 'failed')),
    error_message    TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    acknowledged_at  TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_provisioning_intents_status
    ON provisioning_intents(status, created_at);

-- Nullable, deliberately. See above: there is nothing to put in them.
ALTER TABLE shadow_credentials
    ADD COLUMN IF NOT EXISTS credential_hash TEXT,
    ADD COLUMN IF NOT EXISTS algorithm       TEXT,
    ADD COLUMN IF NOT EXISTS salt_params     TEXT;

ALTER TABLE shadow_credentials DROP COLUMN IF EXISTS enrolled_before_cutover;

COMMENT ON TABLE shadow_credentials IS NULL;
