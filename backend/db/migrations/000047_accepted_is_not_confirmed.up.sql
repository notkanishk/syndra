-- Two facts where the row held one.
--
-- `applied` has always meant "Zitadel returned 2xx". That is an acknowledgement
-- of RECEIPT, and the product has been reading it as evidence of STATE — which
-- is how a row could sit `applied` for a grant Zitadel had never been asked
-- for. That particular defect is fixed upstream (nothing may skip the call any
-- more), but the vocabulary that let it render as success is still here.
--
-- `confirmed_at` is when a read of Zitadel actually OBSERVED the change. It is
-- deliberately nullable and deliberately not part of `status`: accepted and
-- unconfirmed is a real, ordinary, temporary state, not a fault. Zitadel's read
-- path is a projection over its eventstore, so a read issued a millisecond
-- after an accepted write can legitimately not see it yet.
--
-- What makes it a finding is AGE, not absence. A row still unconfirmed long
-- after it was accepted is the one worth a person's attention, and until now
-- there was no column in which that question could even be asked.
ALTER TABLE propagation_outbox
    ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ;

-- The rows this is asked of are the settled ones, and the question is always
-- "applied, and not yet seen". Partial, because unapplied rows are a different
-- queue with its own index.
CREATE INDEX IF NOT EXISTS idx_outbox_applied_unconfirmed
    ON propagation_outbox (completed_at)
 WHERE status = 'applied' AND confirmed_at IS NULL;
