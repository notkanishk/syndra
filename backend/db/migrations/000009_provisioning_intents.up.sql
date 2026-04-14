CREATE TABLE IF NOT EXISTS provisioning_intents (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    target_uid      TEXT        NOT NULL CHECK (btrim(target_uid) <> ''),
    action          TEXT        NOT NULL CHECK (action IN ('add', 'remove')),
    lldap_group     TEXT        NOT NULL CHECK (btrim(lldap_group) <> ''),
    source_project  TEXT        NOT NULL CHECK (btrim(source_project) <> ''),
    source_role     TEXT        NOT NULL CHECK (btrim(source_role) <> ''),
    webhook_event_id UUID,
    idempotency_key TEXT        NOT NULL UNIQUE,
    status          TEXT        NOT NULL DEFAULT 'pending'
                                    CHECK (status IN ('pending', 'acknowledged', 'completed', 'failed')),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_provisioning_intents_target ON provisioning_intents(target_uid);
CREATE INDEX IF NOT EXISTS idx_provisioning_intents_status ON provisioning_intents(status);
CREATE INDEX IF NOT EXISTS idx_provisioning_intents_created ON provisioning_intents(created_at);
