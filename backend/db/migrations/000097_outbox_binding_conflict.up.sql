-- 000097_outbox_binding_conflict (addon-contract 7t-4)
--
-- A change the add-on applied but Syndra could not attribute (two of Syndra's
-- records disagree about whose account it landed on: db.ErrBindingConflict)
-- settles with its own wait_reason, 'binding_conflict'. Before this it shared
-- 'target_refused', and the pages told the two apart by the wording of
-- last_error: wrong the day the system is renamed, and the misreading it
-- invites is a re-send onto the same account. Like 'target_refused' it makes the
-- person's next withdrawal wait for an operator (db.runnerSends reads both).
ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_wait_reason_check;
ALTER TABLE propagation_outbox ADD CONSTRAINT propagation_outbox_wait_reason_check
    CHECK (wait_reason IS NULL OR wait_reason IN ('role_waits', 'manual', 'target_refused', 'binding_conflict')
           OR wait_reason LIKE 'halted:%' OR wait_reason LIKE 'refused:%');
