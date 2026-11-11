-- 000092_addon_operation_no_change, reversed.
--
-- A backend before 000092 read every 2xx as succeeded, so a no_change row is,
-- in its words, succeeded: the add-on answered and nothing is in doubt. Folded
-- so the narrower CHECK can be restored; the audit row each call wrote at
-- dispatch is untouched.
UPDATE addon_operations SET status = 'succeeded' WHERE status = 'no_change';
ALTER TABLE addon_operations DROP CONSTRAINT IF EXISTS addon_operations_status_check;
ALTER TABLE addon_operations ADD CONSTRAINT addon_operations_status_check
    CHECK (status IN ('dispatching', 'succeeded', 'rejected', 'unreached', 'indeterminate'));
