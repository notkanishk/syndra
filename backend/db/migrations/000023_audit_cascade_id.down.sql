-- Reverting drops the lineage link. The audit rows themselves survive; only
-- their pointer into the cascade that carried them out is lost, and it is not
-- recoverable afterwards (see the up migration on why timestamp proximity is
-- not a backfill).
DROP INDEX IF EXISTS idx_audit_logs_cascade_id;

ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS cascade_id;
