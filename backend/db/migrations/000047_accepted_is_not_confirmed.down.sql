-- Reverts 000047. Dropping the column loses which writes had been observed;
-- nothing else depends on it, and a re-application simply starts confirming
-- again from the next drain.
DROP INDEX IF EXISTS idx_outbox_applied_unconfirmed;

ALTER TABLE propagation_outbox
    DROP COLUMN IF EXISTS confirmed_at;
