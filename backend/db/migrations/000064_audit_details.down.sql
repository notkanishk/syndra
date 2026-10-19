DROP INDEX IF EXISTS idx_audit_logs_cancelled_unsent_target;
ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS details;
