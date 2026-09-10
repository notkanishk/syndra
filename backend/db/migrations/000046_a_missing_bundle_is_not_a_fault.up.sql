-- Five real accounts (4 Aug, 6 Aug, 18 Aug, 19 Aug, 9 Sep) hit
-- ErrNoWelcomeBundleConfigured and were recorded as status='failed'. Nothing
-- about that outcome is a fault: nobody's write errored, nothing needs a
-- retry, and the only truthful statement is "no default bundle exists to
-- give them". Filing it as 'failed' put five clean rows in the same bucket
-- as a real DB fault, which is exactly how an operator learns to stop
-- reading a queue.
--
-- 'unconfigured' says what happened without asserting a defect. It is
-- deliberately NOT a claim about intent — Syndra cannot tell "this deployment
-- doesn't want auto-onboarding" from "somebody forgot to flag one" from a
-- missing row, and does not pretend to. What it can promise is that this
-- status is never a defect report, so an operator who fixes it or leaves it
-- alone is making a choice, not clearing an alarm.
--
-- Same shape as 000013's dropped_enrichment_incomplete on webhook_events: a
-- third outcome for "received and understood, not acted on", not a rename of
-- 'failed'.
ALTER TABLE onboarding_triggers
    DROP CONSTRAINT IF EXISTS ck_onboarding_triggers_status;

ALTER TABLE onboarding_triggers
    ADD CONSTRAINT ck_onboarding_triggers_status
        CHECK (status IN ('pending', 'completed', 'failed', 'unconfigured'));

-- And the five rows this was written for.
--
-- Widening the vocabulary changes nothing on its own: the accounts from 4 Aug,
-- 6 Aug, 18 Aug, 19 Aug and 9 Sep would keep reading as five defects, which is
-- the exact queue this change exists to empty. Matched on the recorded cause
-- rather than on status alone, so a row that failed for any OTHER reason keeps
-- saying so.
UPDATE onboarding_triggers
   SET status = 'unconfigured'
 WHERE status = 'failed'
   AND error_message LIKE '%no welcome bundle configured%';
