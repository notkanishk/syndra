-- Dropping the column takes the background drain's ability to claim add-on
-- withdrawals with it: those rows go back to waiting for an operator, which is
-- where they were before this migration and is safe, merely slow.
DROP INDEX IF EXISTS idx_outbox_pending_withdrawals;
ALTER TABLE propagation_outbox
    DROP CONSTRAINT IF EXISTS propagation_outbox_withdraws_only_check;
ALTER TABLE propagation_outbox
    DROP COLUMN IF EXISTS withdraws_only;
