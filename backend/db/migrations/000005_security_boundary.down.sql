-- Reverse migration 005

DROP TABLE IF EXISTS onboarding_triggers;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'claim_profiles' AND column_name = 'claim_failure_mode'
    ) THEN
        ALTER TABLE claim_profiles
            DROP COLUMN IF EXISTS minimal_safe_claims,
            DROP COLUMN IF EXISTS claim_failure_mode;
    END IF;
END $$;
