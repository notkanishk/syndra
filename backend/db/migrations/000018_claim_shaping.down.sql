-- Reverses 000018_claim_shaping.

DROP INDEX IF EXISTS idx_app_claim_overrides_project;
DROP INDEX IF EXISTS idx_app_claim_overrides_project_claim;
DROP TABLE IF EXISTS app_claim_overrides;

ALTER TABLE claim_profiles
    DROP COLUMN IF EXISTS attribute_claims,
    DROP COLUMN IF EXISTS static_claims,
    DROP COLUMN IF EXISTS updated_at;
