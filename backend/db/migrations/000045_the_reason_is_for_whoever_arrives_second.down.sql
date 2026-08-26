-- Restores the constraint without the reason, then drops the column.
ALTER TABLE target_merge_findings
    DROP CONSTRAINT IF EXISTS merge_finding_decision_is_attributed;

ALTER TABLE target_merge_findings
    ADD CONSTRAINT merge_finding_decision_is_attributed
    CHECK ((decided_at IS NULL AND decided_by IS NULL AND decision IS NULL)
        OR (decided_at IS NOT NULL AND btrim(decided_by) <> '' AND decision IS NOT NULL));

ALTER TABLE target_merge_findings
    DROP COLUMN IF EXISTS decision_reason;
