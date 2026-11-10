-- 000086_outbox_target_refused
--
-- An add-on's deterministic refusal (it rejected the call, the plan blocked
-- the subject, or the account's name is held by another) now settles with
-- wait_reason 'target_refused', so the background runner can tell a refusal
-- from a send that ran out of retries against an unreachable target: only the
-- refusal makes the person's next withdrawal wait for an operator
-- (db.runnerSends). Not 'refused:<cause>', which the screens say as Zitadel's.
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual', 'target_refused')
           OR wait_reason LIKE 'halted:%' OR wait_reason LIKE 'refused:%');
