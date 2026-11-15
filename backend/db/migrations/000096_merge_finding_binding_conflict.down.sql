-- Back to the 000042 check. A binding_conflict finding has no outcome the old
-- check allows, so its rows go first: the next sweep after a re-upgrade raises
-- any conflict the add-on still reports again.
DELETE FROM target_merge_findings WHERE outcome = 'binding_conflict';
ALTER TABLE target_merge_findings DROP CONSTRAINT IF EXISTS target_merge_findings_outcome_check;
ALTER TABLE target_merge_findings ADD CONSTRAINT target_merge_findings_outcome_check
    CHECK (outcome IN ('theirs_only', 'conflict', 'deleted_upstream'));
