-- 000100_merge_finding_adopted_kept (addon-contract 7t-7)
--
-- Two more answers to a binding_conflict finding (owner ruling 17 Nov 2026),
-- besides 'unbound' (stop tracking):
--   'adopted' the holder is theirs: account.adopt moves the person's binding
--             to the account that holds its name or number;
--   'kept'    claimed twice: the account stays this person's, and the other
--             person's binding is released (account.release).
-- Each is a decision first (the decision column has no value check) and the
-- row closes carrying it when the add-on stops reporting the conflict
-- (ClearMergeFindingsOutside writes COALESCE(decision, 'agreed')), so the
-- resolution check has to allow both. Widened, every earlier value kept; the
-- constraint keeps its name from 000042.
ALTER TABLE target_merge_findings DROP CONSTRAINT IF EXISTS merge_finding_resolution_is_known;
ALTER TABLE target_merge_findings ADD CONSTRAINT merge_finding_resolution_is_known
    CHECK (resolution IS NULL OR resolution IN ('keep_ours', 'take_theirs', 'reprovisioned', 'unbound', 'agreed', 'adopted', 'kept'));
