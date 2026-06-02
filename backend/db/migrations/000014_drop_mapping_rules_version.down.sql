-- Rollback: restore the column AND the ck_mapping_rules_version_positive
-- CHECK constraint that 000004_contract_hardening added, so rolling back
-- 14 → 13 leaves the schema at exactly the contract migration 13 expects.
-- Pre-drop version values are not recoverable (audit_logs never stored them);
-- every restored row gets DEFAULT 1, which satisfies the >0 constraint.

ALTER TABLE mapping_rules ADD COLUMN version INT NOT NULL DEFAULT 1;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_mapping_rules_version_positive') THEN
        ALTER TABLE mapping_rules
        ADD CONSTRAINT ck_mapping_rules_version_positive CHECK (version > 0);
    END IF;
END $$;
