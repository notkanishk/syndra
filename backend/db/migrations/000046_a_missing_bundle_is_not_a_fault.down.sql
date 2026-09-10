-- Reverts 000046.
--
-- The rows are moved BACK to 'failed', never deleted. An earlier draft of this
-- dropped them to satisfy the narrower constraint, which trades a record that
-- five real people were onboarded-attempted for a schema that fits — and the
-- rows are the only evidence anywhere that those attempts happened. A rollback
-- may restore a worse vocabulary; it may not destroy history to do it.
UPDATE onboarding_triggers SET status = 'failed' WHERE status = 'unconfigured';

ALTER TABLE onboarding_triggers
    DROP CONSTRAINT IF EXISTS ck_onboarding_triggers_status;

ALTER TABLE onboarding_triggers
    ADD CONSTRAINT ck_onboarding_triggers_status
        CHECK (status IN ('pending', 'completed', 'failed'));
