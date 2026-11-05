-- 000083_outbox_refused_cause
--
-- A row Zitadel refused outright now keeps the plain cause (not_allowed,
-- not_found, invalid, other) as wait_reason "refused:<cause>", so Pending
-- changes and Home can say why Syndra stopped sending it. wait_reason already
-- carries this vocabulary in results (db.WaitReasonRefused); only its check
-- constraint (000056) refused the value on a stored row.
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual')
           OR wait_reason LIKE 'halted:%' OR wait_reason LIKE 'refused:%');
