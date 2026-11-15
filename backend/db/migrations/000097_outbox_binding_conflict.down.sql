-- Back to the 000086 check. Such a row was 'target_refused' before 000097 (the
-- same hold on the person's next withdrawal), so that is what it becomes.
UPDATE propagation_outbox SET wait_reason = 'target_refused' WHERE wait_reason = 'binding_conflict';
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual', 'target_refused')
           OR wait_reason LIKE 'halted:%' OR wait_reason LIKE 'refused:%');
