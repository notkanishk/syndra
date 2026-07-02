-- 000015_zitadel_propagation_outbox.up.sql
-- Wave 2 · Part 4 (B4/D3): the outbox buffer for MkAuth-mediated Zitadel grant
-- mutations, plus source attribution on direct_role_grants. The full 5-value
-- source enum is installed now so sub-phase 3 (cascade) needs no further ALTER.
-- `applied` is terminal success; there is no `confirmed` state (design Decision 1).

CREATE TABLE IF NOT EXISTS pending_zitadel_propagations (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    op_type          TEXT NOT NULL CHECK (op_type IN ('add', 'revoke', 'replace')),
    user_id          VARCHAR(255) NOT NULL,
    project_id       VARCHAR(255) NOT NULL,
    role_keys        TEXT[] NOT NULL,
    zitadel_grant_id TEXT,
    payload_json     JSONB NOT NULL,
    idempotency_key  UUID NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'in_flight', 'applied', 'failed')),
    attempts         INT NOT NULL DEFAULT 0,
    last_error       TEXT,
    initiated_by     VARCHAR(255) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pending_zitadel_propagations_status
    ON pending_zitadel_propagations(status, created_at);

ALTER TABLE direct_role_grants
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'direct'
        CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));

ALTER TABLE direct_role_grants
    ADD COLUMN IF NOT EXISTS source_ref TEXT;
