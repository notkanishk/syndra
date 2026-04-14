-- Webhook event persistence for deduplication, audit, and operator visibility.
CREATE TABLE IF NOT EXISTS webhook_events (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type      TEXT        NOT NULL,
    user_id         TEXT        NOT NULL CHECK (btrim(user_id) <> ''),
    source_project  TEXT        NOT NULL CHECK (btrim(source_project) <> ''),
    role_key        TEXT,
    idempotency_key TEXT        NOT NULL UNIQUE,
    status          TEXT        NOT NULL DEFAULT 'received'
                                    CHECK (status IN ('received', 'processed', 'failed')),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_user ON webhook_events(user_id);
CREATE INDEX IF NOT EXISTS idx_webhook_events_status ON webhook_events(status);
