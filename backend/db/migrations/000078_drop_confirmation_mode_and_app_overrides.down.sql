-- 000078 down: the schema comes back, the values do not. Every bundle and rule
-- returns as 'auto', which is what roles that wait already made them, and the
-- overrides table returns empty, which is what it was when it was dropped.
ALTER TABLE mapping_rules
    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));
ALTER TABLE bundles
    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));
INSERT INTO config_settings (key, value, updated_by)
    VALUES ('global.default_rule_confirmation_mode', 'auto', 'migration')
    ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS app_claim_overrides (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id VARCHAR(255) UNIQUE NOT NULL,
    zitadel_project_id VARCHAR(255) NOT NULL,
    claim_name VARCHAR(128) NOT NULL,
    format_type VARCHAR(64) NOT NULL,
    attribute_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
    static_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_app_claim_overrides_format_type
        CHECK (format_type IN ('array', 'csv', 'space_delimited'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_app_claim_overrides_project_claim
    ON app_claim_overrides(zitadel_project_id, claim_name);
CREATE INDEX IF NOT EXISTS idx_app_claim_overrides_project
    ON app_claim_overrides(zitadel_project_id);
