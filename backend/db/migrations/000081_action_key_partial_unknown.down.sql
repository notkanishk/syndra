-- Back to five outcomes: the two new ones were "failed" before 000081.
UPDATE action_key_rotations SET outcome = 'failed' WHERE outcome IN ('partial', 'unknown');
ALTER TABLE action_key_rotations DROP CONSTRAINT IF EXISTS action_key_rotations_outcome_check;
ALTER TABLE action_key_rotations ADD CONSTRAINT action_key_rotations_outcome_check
    CHECK (outcome IN ('in_flight', 'verified', 'unverified', 'failed', 'abandoned'));
