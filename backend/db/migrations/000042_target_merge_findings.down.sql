-- 000042_target_merge_findings.down.sql
--
-- Dropping this discards standing findings — differences a person was asked to
-- decide about and had not yet. The differences themselves survive on the
-- target, so the next sweep re-detects and re-raises them; what is lost is when
-- each was first seen and anything already decided.
DROP INDEX IF EXISTS idx_merge_finding_by_target;
DROP INDEX IF EXISTS idx_merge_finding_open;
DROP TABLE IF EXISTS target_merge_findings;
