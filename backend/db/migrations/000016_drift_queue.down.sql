-- 000016_drift_queue.down.sql
-- Reverses 000016. drift_items is ephemeral triage state and exclusions are an
-- operator convenience list; both are re-derivable by the next reconciliation
-- sweep, so dropping them loses no canonical record.

DROP TABLE IF EXISTS external_grant_exclusions;
DROP INDEX IF EXISTS idx_drift_items_pending_unique;
DROP INDEX IF EXISTS idx_drift_items_status;
DROP TABLE IF EXISTS drift_items;
