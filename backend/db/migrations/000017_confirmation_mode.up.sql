-- Wave 2 · Part 4 — Sub-phase 3: per-source confirmation mode for cascade projection.
-- mapping_rules + bundles gain confirmation_mode (auto drains immediately, manual queues).
-- config_settings holds the global default new rules/bundles inherit.
-- pending_zitadel_propagations gains source/source_ref so a cascade outbox row records its
-- originating bundle/rule (000015 outbox had no source column) — used for "Recent automated
-- cascades" and for grouping the Pending UI by source.

ALTER TABLE mapping_rules
    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));

ALTER TABLE bundles
    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));

-- Attribution on the outbox row (NOT direct_role_grants — cascades do not write the ledger;
-- see design pivot). Default 'direct' keeps existing operator rows valid; source_ref is the
-- bundle/rule id for cascade rows, NULL otherwise. Full 5-value enum matches direct_role_grants.
ALTER TABLE pending_zitadel_propagations
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'direct'
        CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));
ALTER TABLE pending_zitadel_propagations
    ADD COLUMN IF NOT EXISTS source_ref TEXT;
CREATE INDEX IF NOT EXISTS idx_pending_zitadel_propagations_source
    ON pending_zitadel_propagations(source, created_at);

CREATE TABLE IF NOT EXISTS config_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255)
);

INSERT INTO config_settings (key, value, updated_by)
    VALUES ('global.default_rule_confirmation_mode', 'auto', 'migration')
    ON CONFLICT (key) DO NOTHING;
