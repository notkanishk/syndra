-- 000080_outbox_not_before
--
-- A requeued outbox row waits before it is tried again. RequeuePropagation
-- sets not_before from extcall.Backoff (jittered, doubling, capped); both
-- batch claims skip a row until then. NULL = claimable now (every row
-- written before this, and every row never requeued).
ALTER TABLE propagation_outbox ADD COLUMN IF NOT EXISTS not_before TIMESTAMPTZ;
