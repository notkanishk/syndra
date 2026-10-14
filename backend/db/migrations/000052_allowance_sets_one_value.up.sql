-- An allowance may now SET one field for one person (truenas-reconcile 10,
-- owner 14 Oct 2026): an operator raising one member's home folder limit over
-- what their roles give. Only fields the add-on declares per-person may take
-- one; the handler checks that against the manifest, which this table cannot.
--
-- Bounded like a denial, by an expiry or a review date: an exception for one
-- person is a decision somebody should be prompted to revisit.
ALTER TABLE allowances DROP CONSTRAINT IF EXISTS allowances_direction_check;
ALTER TABLE allowances
    ADD CONSTRAINT allowances_direction_check CHECK (direction IN ('allow', 'deny', 'set'));
