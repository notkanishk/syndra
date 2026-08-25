-- 000043_a_decision_is_not_a_settlement.down.sql
--
-- Rolling this back loses which findings have been decided but not yet applied.
-- Those rows return to looking undecided, so an operator is asked a question
-- somebody has already answered — and the convergence their answer queued still
-- lands, which is the confusing half.
DROP INDEX IF EXISTS idx_merge_finding_decided;
ALTER TABLE target_merge_findings DROP CONSTRAINT IF EXISTS merge_finding_decision_is_attributed;
ALTER TABLE target_merge_findings
    DROP COLUMN IF EXISTS decision,
    DROP COLUMN IF EXISTS decided_by,
    DROP COLUMN IF EXISTS decided_at;
