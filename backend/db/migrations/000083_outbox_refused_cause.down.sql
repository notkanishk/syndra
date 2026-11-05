-- Back to the 000056 check: a refusal's cause was not stored before 000083.
UPDATE propagation_outbox SET wait_reason = NULL WHERE wait_reason LIKE 'refused:%';
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual') OR wait_reason LIKE 'halted:%');
