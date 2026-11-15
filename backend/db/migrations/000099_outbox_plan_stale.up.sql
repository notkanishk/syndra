-- 000099_outbox_plan_stale (addon-contract 7t-6)
--
-- An add-on row the add-on refused with PLAN_STALE (the person's account moved
-- on the system after the change was approved) settles with its own
-- wait_reason, 'plan_stale'. Before this it settled with none and a
-- code-prefixed sentence in last_error, so the pages could only say "{system}
-- refused it". Like a failed row with no reason it holds nothing: a fresh
-- plan may well be accepted, so the person's next withdrawal does not wait
-- for an operator (db.runnerSends reads 'target_refused' and
-- 'binding_conflict' only).
--
-- 'binding_conflict' keeps its value and now means the add-on refused or
-- blocked the change because the person's binding names no account (its
-- conflict, raised as the binding_conflict finding under Waiting on you). The
-- meaning 000097 gave it, a change applied that Syndra's mirror could not
-- attribute, cannot arise since 000098: the mirror follows the add-on.
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual', 'target_refused', 'binding_conflict', 'plan_stale')
           OR wait_reason LIKE 'halted:%' OR wait_reason LIKE 'refused:%');
