-- NOT VALID: rows recorded as 'set' stay, as history, and only new rows are held
-- to the narrower rule.
ALTER TABLE allowances DROP CONSTRAINT IF EXISTS allowances_direction_check;
ALTER TABLE allowances
    ADD CONSTRAINT allowances_direction_check CHECK (direction IN ('allow', 'deny')) NOT VALID;
