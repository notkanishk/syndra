-- Back to the 000097 check. A 'plan_stale' row settled with no wait_reason
-- before 000099 (a failed row that holds nothing), so that is what it becomes;
-- its last_error keeps the plain sentence.
UPDATE propagation_outbox SET wait_reason = NULL WHERE wait_reason = 'plan_stale';
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual', 'target_refused', 'binding_conflict')
           OR wait_reason LIKE 'halted:%' OR wait_reason LIKE 'refused:%');
