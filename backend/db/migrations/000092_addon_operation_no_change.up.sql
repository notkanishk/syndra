-- 000092_addon_operation_no_change (addon-contract 6.3a)
--
-- An operation can answer that it changed nothing (outcome no_change, feature
-- calls: an item the member already holds), and the record says so rather
-- than `succeeded`, so a no-op is never recorded or counted as done. Claimed
-- like `succeeded` (addon_operations_claim_check already requires a claim for
-- every status but dispatching and unreached) and settled like any terminal
-- status (addon_operations_settled_check).
--
-- Expand-only: the CHECK only widens, every existing row satisfies it, and an
-- older backend never writes the new word. The constraint is the one Postgres
-- named for 000027's inline CHECK.
ALTER TABLE addon_operations DROP CONSTRAINT IF EXISTS addon_operations_status_check;
ALTER TABLE addon_operations ADD CONSTRAINT addon_operations_status_check
    CHECK (status IN ('dispatching', 'succeeded', 'rejected', 'unreached', 'indeterminate', 'no_change'));
