-- Back to the 000042 check. A standing decision an older backend cannot name
-- is withdrawn (the finding stands undecided, and the sweep still closes it
-- when the add-on stops reporting the conflict); the work it did on the target
-- is done and stays done. A settled one reads 'agreed', the closing an older
-- backend writes when nobody's choice is on the row; its decision column (no
-- value check) keeps what was chosen, by whom and why.
UPDATE target_merge_findings
SET decision = NULL, decided_by = NULL, decided_at = NULL, decision_reason = NULL
WHERE resolved_at IS NULL AND decision IN ('adopted', 'kept');
UPDATE target_merge_findings SET resolution = 'agreed' WHERE resolution IN ('adopted', 'kept');
ALTER TABLE target_merge_findings DROP CONSTRAINT IF EXISTS merge_finding_resolution_is_known;
ALTER TABLE target_merge_findings ADD CONSTRAINT merge_finding_resolution_is_known
    CHECK (resolution IS NULL OR resolution IN ('keep_ours', 'take_theirs', 'reprovisioned', 'unbound', 'agreed'));
