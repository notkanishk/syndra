-- 000018_claim_shaping
--
-- Makes the token shape an operator-editable artefact instead of a fixed
-- literal in the code. Two additions:
--
--   1. claim_profiles gains attribute_claims + static_claims, so a project's
--      token can carry more than the roles array (email, team, a tenant
--      constant) without a code change.
--   2. app_claim_overrides lets one application on a shared project receive a
--      different claim key/shape from its siblings.
--
-- Zitadel's Actions v2 function payload carries no application identifier, so
-- a token issued for a project carries the project default AND every override
-- key configured on that project; each application reads its own key. Claim
-- keys are therefore validated unique per project (partially here, fully in
-- the service layer, which can see both tables at once).

ALTER TABLE claim_profiles
    ADD COLUMN IF NOT EXISTS attribute_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS static_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;

CREATE TABLE IF NOT EXISTS app_claim_overrides (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id VARCHAR(255) UNIQUE NOT NULL,
    zitadel_project_id VARCHAR(255) NOT NULL,
    claim_name VARCHAR(128) NOT NULL,
    format_type VARCHAR(64) NOT NULL,
    attribute_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
    static_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_app_claim_overrides_format_type') THEN
        ALTER TABLE app_claim_overrides
        ADD CONSTRAINT ck_app_claim_overrides_format_type
        CHECK (format_type IN ('array', 'csv', 'space_delimited'));
    END IF;
END $$;

-- Two applications on the same project may not claim the same key: a flat JWT
-- cannot hold two values for one name. The project default is checked against
-- this set in the service layer.
CREATE UNIQUE INDEX IF NOT EXISTS idx_app_claim_overrides_project_claim
    ON app_claim_overrides(zitadel_project_id, claim_name);

CREATE INDEX IF NOT EXISTS idx_app_claim_overrides_project
    ON app_claim_overrides(zitadel_project_id);
