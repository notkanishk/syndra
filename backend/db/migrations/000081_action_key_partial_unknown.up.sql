-- 000081_action_key_partial_unknown
--
-- Two more outcomes for a key replacement (coherence audit W4):
--   partial : Zitadel replaced one target's key and not the other's
--   unknown : Zitadel's answer was lost and the read-back failed, so nobody
--             can say whether the key was replaced
-- "failed" now means only that Zitadel replaced nothing.
--
-- 000077 declared the CHECK inline on the column, so Postgres named it
-- <table>_<column>_check.
ALTER TABLE action_key_rotations DROP CONSTRAINT IF EXISTS action_key_rotations_outcome_check;
ALTER TABLE action_key_rotations ADD CONSTRAINT action_key_rotations_outcome_check
    CHECK (outcome IN ('in_flight', 'verified', 'unverified', 'partial', 'unknown', 'failed', 'abandoned'));
