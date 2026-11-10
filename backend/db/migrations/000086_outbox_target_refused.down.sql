-- Back to the 000083 check: an add-on refusal was not marked before 000086.
UPDATE propagation_outbox SET wait_reason = NULL WHERE wait_reason = 'target_refused';
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual')
           OR wait_reason LIKE 'halted:%' OR wait_reason LIKE 'refused:%');
